package voicepack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"nstudio/app/common/response"
	"nstudio/app/tts/engine"
)

type Pack struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Version string  `json:"version,omitempty"`
	Engine  string  `json:"engine"`
	Voices  []Voice `json:"voices"`

	PackPath string `json:"-"`
}

type Voice struct {
	File   string `json:"file"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
	ID     string `json:"id,omitempty"`
}

func (voice Voice) VoiceID() string {
	if voice.ID != "" {
		return voice.ID
	}
	return strings.TrimSuffix(voice.File, filepath.Ext(voice.File))
}

func PacksRootPath(engineID string) (string, error) {
	if runtime.GOOS == "windows" {
		userHomePath, err := os.UserHomeDir()
		if err != nil {
			return "", response.Err("%w", err)
		}
		return filepath.Join(userHomePath, "Narration Studio", "voices", engineID), nil
	}
	userConfigPath, err := os.UserConfigDir()
	if err != nil {
		return "", response.Err("%w", err)
	}
	return filepath.Join(userConfigPath, "Narration Studio", "voices", engineID), nil
}

func ScanInstalled(engineID string) ([]Pack, error) {
	rootPath, err := PacksRootPath(engineID)
	if err != nil {
		return nil, response.Err("%w", err)
	}
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var packs []Pack
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packPath := filepath.Join(rootPath, entry.Name())
		rawPack, err := os.ReadFile(filepath.Join(packPath, "pack.json"))
		if err != nil {
			continue
		}
		var pack Pack
		if err := json.Unmarshal(rawPack, &pack); err != nil {
			continue
		}
		if pack.ID == "" {
			pack.ID = entry.Name()
		}
		pack.PackPath = packPath
		packs = append(packs, pack)
	}
	return packs, nil
}

func AllVoices(engineID string) ([]engine.Voice, error) {
	packs, err := ScanInstalled(engineID)
	if err != nil {
		return nil, response.Err("%w", err)
	}
	var voices []engine.Voice
	for _, pack := range packs {
		for _, voice := range pack.Voices {
			voices = append(voices, engine.Voice{
				ID:     pack.ID + "/" + voice.VoiceID(),
				Name:   voice.Name,
				Gender: voice.Gender,
			})
		}
	}
	return voices, nil
}

func ResolvePath(engineID, voiceID string) (string, bool) {
	packs, err := ScanInstalled(engineID)
	if err != nil {
		return "", false
	}

	requestedPackID, requestedVoiceID, hasSlash := strings.Cut(voiceID, "/")
	if !hasSlash {
		requestedVoiceID = requestedPackID
		requestedPackID = ""
	}

	for _, pack := range packs {
		if requestedPackID != "" && pack.ID != requestedPackID {
			continue
		}
		for _, voice := range pack.Voices {
			if voice.VoiceID() != requestedVoiceID {
				continue
			}
			absoluteVoicePath := filepath.Join(pack.PackPath, filepath.FromSlash(voice.File))
			if _, err := os.Stat(absoluteVoicePath); err == nil {
				return absoluteVoicePath, true
			}
		}
	}
	return "", false
}
