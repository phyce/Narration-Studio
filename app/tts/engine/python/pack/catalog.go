package pack

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"nstudio/app/common/response"
	"nstudio/app/tts/engine/python"
)

var CatalogURL = "https://api.github.com/repos/phyce/NarrationStudio-Engines/releases"

type AvailablePack struct {
	Manifest   python.Manifest `json:"manifest"`
	BundleURL  string          `json:"bundleUrl"`
	BundleSize int64           `json:"bundleSize"`
	Installed  bool            `json:"installed"`
	TagName    string          `json:"tagName"`
	// ExpectedSHA256 is the lowercase hex SHA-256 digest the downloaded bundle
	// must match before it is extracted/executed. It is populated from the
	// GitHub asset "digest" field ("sha256:<hex>") when the API exposes it, and
	// is empty for older API responses or local packs that carry no trusted
	// digest.
	ExpectedSHA256 string `json:"expectedSha256"`
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
		// Digest is provided by newer GitHub Releases API responses in the
		// form "sha256:<hex>". Used to verify bundle integrity before install.
		Digest string `json:"digest"`
	} `json:"assets"`
}

func FetchAvailable() ([]AvailablePack, error) {
	httpClient := &http.Client{Timeout: 20 * time.Second}
	catalogRequest, err := http.NewRequest(http.MethodGet, CatalogURL, nil)
	if err != nil {
		return nil, response.Err("%w", err)
	}
	catalogRequest.Header.Set("Accept", "application/vnd.github+json")

	catalogResponse, err := httpClient.Do(catalogRequest)
	if err != nil {
		return nil, response.Err("%w", err)
	}
	defer catalogResponse.Body.Close()

	if catalogResponse.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(catalogResponse.Body)
		return nil, response.Err("github API returned %d: %s", catalogResponse.StatusCode, string(errorBody))
	}

	var releases []githubRelease
	if err := json.NewDecoder(catalogResponse.Body).Decode(&releases); err != nil {
		return nil, response.Err("%w", err)
	}

	platformSuffix := strings.ToLower(fmt.Sprintf("-%s-%s", runtime.GOOS, runtime.GOARCH))

	var availablePacks []AvailablePack
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}

		var bundleURL, metadataURL, bundleDigest string
		var bundleSize int64

		for _, asset := range release.Assets {
			lowerCaseAssetName := strings.ToLower(asset.Name)
			if !strings.Contains(lowerCaseAssetName, platformSuffix) {
				continue
			}
			switch {
			case strings.HasSuffix(lowerCaseAssetName, ".nstudio"):
				bundleURL = asset.BrowserDownloadURL
				bundleSize = asset.Size
				bundleDigest = asset.Digest
			case strings.HasSuffix(lowerCaseAssetName, ".metadata.json"):
				metadataURL = asset.BrowserDownloadURL
			}
		}

		if bundleURL == "" || metadataURL == "" {
			continue
		}

		manifest, err := fetchMetadata(metadataURL)
		if err != nil {
			continue
		}

		availablePacks = append(availablePacks, AvailablePack{
			Manifest:       *manifest,
			BundleURL:      bundleURL,
			BundleSize:     bundleSize,
			Installed:      IsInstalled(manifest.ID),
			TagName:        release.TagName,
			ExpectedSHA256: parseSHA256Digest(bundleDigest),
		})
	}

	return availablePacks, nil
}

// parseSHA256Digest extracts the lowercase hex payload from a GitHub asset
// digest of the form "sha256:<hex>". Any other/empty value yields "", which the
// installer treats as "no trusted digest available".
func parseSHA256Digest(digest string) string {
	digest = strings.TrimSpace(digest)
	const prefix = "sha256:"
	if !strings.HasPrefix(strings.ToLower(digest), prefix) {
		return ""
	}
	return strings.ToLower(digest[len(prefix):])
}

func fetchMetadata(metadataURL string) (*python.Manifest, error) {
	httpClient := &http.Client{Timeout: 20 * time.Second}
	metadataResponse, err := httpClient.Get(metadataURL)
	if err != nil {
		return nil, response.Err("%w", err)
	}
	defer metadataResponse.Body.Close()
	if metadataResponse.StatusCode != http.StatusOK {
		return nil, response.Err("metadata %s: status %d", metadataURL, metadataResponse.StatusCode)
	}
	var manifest python.Manifest
	if err := json.NewDecoder(metadataResponse.Body).Decode(&manifest); err != nil {
		return nil, response.Err("%w", err)
	}
	return &manifest, nil
}
