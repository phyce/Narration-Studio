package piper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const piperDownloadProgressEvent = "piper.download.progress"

type DownloadProgress struct {
	ModelID    string `json:"modelId"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percent    int    `json:"percent"`
	Done       bool   `json:"done"`
}

type progressReader struct {
	r          io.Reader
	modelID    string
	downloaded int64
	total      int64
	lastEmit   time.Time
	lastPct    int
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 {
		p.downloaded += int64(n)
		p.maybeEmit(false)
	}
	return n, err
}

func (p *progressReader) maybeEmit(force bool) {
	pct := 0
	if p.total > 0 {
		pct = int(p.downloaded * 100 / p.total)
		if pct > 100 {
			pct = 100
		}
	}

	// Throttle to at most ~10 emits/sec and only on percent change, unless forced.
	if !force && pct == p.lastPct && time.Since(p.lastEmit) < 100*time.Millisecond {
		return
	}

	p.lastPct = pct
	p.lastEmit = time.Now()

	eventManager.GetInstance().EmitEvent(piperDownloadProgressEvent, DownloadProgress{
		ModelID:    p.modelID,
		Downloaded: p.downloaded,
		Total:      p.total,
		Percent:    pct,
		Done:       false,
	})
}

const piperModelsReleasesURL = "https://api.github.com/repos/phyce/PiperModels/releases"

type AvailableModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	TagName     string `json:"tagName"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Voices      int    `json:"voices"`
	ModelURL    string `json:"modelUrl"`
	ConfigURL   string `json:"configUrl"`
	MetadataURL string `json:"metadataUrl"`
	Size        int64  `json:"size"`
	Installed   bool   `json:"installed"`
}

type githubRelease struct {
	Name       string `json:"name"`
	TagName    string `json:"tag_name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// parseReleaseBody extracts description (line 1), language (line 2), and voice
// count (line 3, e.g. "64 Voices") from a release body.
func parseReleaseBody(body string) (description, language string, voices int) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	get := func(i int) string {
		if i < len(lines) {
			return strings.TrimSpace(lines[i])
		}
		return ""
	}
	description = get(0)
	language = get(1)

	voiceLine := get(2)
	voiceLine = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(voiceLine, "Voices"), "voices"))
	if n, err := strconv.Atoi(voiceLine); err == nil {
		voices = n
	}
	return
}

func modelIDFromName(name string) string {
	id := strings.ToLower(strings.TrimSpace(name))
	id = strings.ReplaceAll(id, " ", "-")
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func getModelsDir() (string, error) {
	dir := config.GetEngine().Local.Piper.ModelsDirectory
	if dir == "" {
		return "", fmt.Errorf("piper models directory is not set")
	}
	err, expanded := util.ExpandPath(dir)
	if err != nil {
		return "", err
	}
	return expanded, nil
}

// userModelsDir returns the per-user writable fallback path for Piper models.
// On Windows we use %USERPROFILE%\Narration Studio\piper\models to match the
// other user-data paths (output, cache). Other platforms fall back to
// os.UserConfigDir().
func userModelsDir() (string, error) {
	if runtime.GOOS == "windows" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Narration Studio", "piper", "models"), nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Narration Studio", "piper", "models"), nil
}

func isWritable(dir string) bool {
	if _, err := os.Stat(dir); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".nstudio-writetest-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// ensureWritableModelsDir returns a writable models directory. If the configured
// one isn't writable (e.g. Program Files without admin), it falls back to the
// user config dir and updates the saved config to match.
func ensureWritableModelsDir() (string, error) {
	current, err := getModelsDir()
	if err == nil && isWritable(current) {
		return current, nil
	}

	fallback, err := userModelsDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve writable models directory: %w", err)
	}

	if err := os.MkdirAll(fallback, 0o755); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", fallback, err)
	}

	newCfg := config.Get()
	if newCfg.Engine.Local.Piper.ModelsDirectory != fallback {
		newCfg.Engine.Local.Piper.ModelsDirectory = fallback
		if err := config.Set(newCfg); err != nil {
			return "", fmt.Errorf("failed to update models directory config: %w", err)
		}
	}

	return fallback, nil
}

func isModelInstalled(modelsDir, modelID string) bool {
	folder := filepath.Join(modelsDir, modelID)
	for _, suffix := range []string{".onnx", ".onnx.json", ".metadata.json"} {
		path := filepath.Join(folder, modelID+suffix)
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

func FetchAvailableModels() ([]AvailableModel, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodGet, piperModelsReleasesURL, nil)
	if err != nil {
		return nil, response.Err(err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, response.Err(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, response.Err(fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body)))
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, response.Err(err)
	}

	modelsDir, _ := getModelsDir()

	var result []AvailableModel
	for _, rel := range releases {
		if rel.Draft || rel.Prerelease {
			continue
		}

		displayName := rel.Name
		if displayName == "" {
			displayName = rel.TagName
		}
		id := modelIDFromName(displayName)
		if id == "" {
			continue
		}

		var modelURL, configURL, metadataURL string
		var totalSize int64

		for _, asset := range rel.Assets {
			name := asset.Name
			switch {
			case strings.HasSuffix(name, ".metadata.json"):
				metadataURL = asset.BrowserDownloadURL
				totalSize += asset.Size
			case strings.HasSuffix(name, ".onnx.json"):
				configURL = asset.BrowserDownloadURL
				totalSize += asset.Size
			case strings.HasSuffix(name, ".onnx"):
				modelURL = asset.BrowserDownloadURL
				totalSize += asset.Size
			}
		}

		if modelURL == "" || configURL == "" || metadataURL == "" {
			continue
		}

		installed := false
		if modelsDir != "" {
			installed = isModelInstalled(modelsDir, id)
		}

		description, language, voiceCount := parseReleaseBody(rel.Body)

		result = append(result, AvailableModel{
			ID:          id,
			Name:        displayName,
			TagName:     rel.TagName,
			Description: description,
			Language:    language,
			Voices:      voiceCount,
			ModelURL:    modelURL,
			ConfigURL:   configURL,
			MetadataURL: metadataURL,
			Size:        totalSize,
			Installed:   installed,
		})
	}

	featuredRank := map[string]int{
		"libritts": 0,
		"vctk":     1,
	}
	sort.Slice(result, func(i, j int) bool {
		iRank, iFeatured := featuredRank[result[i].ID]
		jRank, jFeatured := featuredRank[result[j].ID]
		if iFeatured || jFeatured {
			if iFeatured && jFeatured {
				return iRank < jRank
			}
			return iFeatured
		}
		if result[i].Voices != result[j].Voices {
			return result[i].Voices > result[j].Voices
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

func downloadFile(url, destPath string, reader *progressReader) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return response.Err(err)
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return response.Err(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return response.Err(fmt.Errorf("download failed (%d) for %s", resp.StatusCode, url))
	}

	var src io.Reader = resp.Body
	if reader != nil {
		reader.r = resp.Body
		src = reader
	}

	tmpPath := destPath + ".part"
	out, err := os.Create(tmpPath)
	if err != nil {
		return response.Err(err)
	}

	_, err = io.Copy(out, src)
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return response.Err(err)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return response.Err(closeErr)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return response.Err(err)
	}
	return nil
}

func DownloadModel(model AvailableModel) error {
	modelsDir, err := ensureWritableModelsDir()
	if err != nil {
		return response.Err(err)
	}

	folder := filepath.Join(modelsDir, model.ID)
	files := []struct {
		url  string
		name string
	}{
		{model.ModelURL, model.ID + ".onnx"},
		{model.ConfigURL, model.ID + ".onnx.json"},
		{model.MetadataURL, model.ID + ".metadata.json"},
	}

	reader := &progressReader{
		modelID: model.ID,
		total:   model.Size,
	}
	reader.maybeEmit(true)

	for _, f := range files {
		dest := filepath.Join(folder, f.name)
		if err := downloadFile(f.url, dest, reader); err != nil {
			return err
		}
	}

	eventManager.GetInstance().EmitEvent(piperDownloadProgressEvent, DownloadProgress{
		ModelID:    model.ID,
		Downloaded: reader.downloaded,
		Total:      reader.total,
		Percent:    100,
		Done:       true,
	})
	return nil
}

func DeleteModel(modelID string) error {
	modelsDir, err := getModelsDir()
	if err != nil {
		return response.Err(err)
	}

	if modelID == "" {
		return response.Err(fmt.Errorf("model ID is required"))
	}

	folder := filepath.Join(modelsDir, modelID)
	// Safety: ensure the folder is actually a child of the models dir
	absModelsDir, _ := filepath.Abs(modelsDir)
	absFolder, _ := filepath.Abs(folder)
	if !strings.HasPrefix(absFolder, absModelsDir+string(os.PathSeparator)) {
		return response.Err(fmt.Errorf("refusing to delete outside models directory"))
	}

	if _, err := os.Stat(folder); os.IsNotExist(err) {
		return nil
	}

	if err := os.RemoveAll(folder); err != nil {
		return response.Err(err)
	}
	return nil
}
