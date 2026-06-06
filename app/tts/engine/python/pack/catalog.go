package pack

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"nstudio/app/tts/engine/python"
)

var CatalogURL = "https://api.github.com/repos/phyce/NarrationStudio-Engines/releases"

type AvailablePack struct {
	Manifest   python.Manifest `json:"manifest"`
	BundleURL  string          `json:"bundleUrl"`
	BundleSize int64           `json:"bundleSize"`
	Installed  bool            `json:"installed"`
	TagName    string          `json:"tagName"`
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

func FetchAvailable() ([]AvailablePack, error) {
	httpClient := &http.Client{Timeout: 20 * time.Second}
	catalogRequest, err := http.NewRequest(http.MethodGet, CatalogURL, nil)
	if err != nil {
		return nil, err
	}
	catalogRequest.Header.Set("Accept", "application/vnd.github+json")

	catalogResponse, err := httpClient.Do(catalogRequest)
	if err != nil {
		return nil, err
	}
	defer catalogResponse.Body.Close()

	if catalogResponse.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(catalogResponse.Body)
		return nil, fmt.Errorf("github API returned %d: %s", catalogResponse.StatusCode, string(errorBody))
	}

	var releases []githubRelease
	if err := json.NewDecoder(catalogResponse.Body).Decode(&releases); err != nil {
		return nil, err
	}

	platformSuffix := strings.ToLower(fmt.Sprintf("-%s-%s", runtime.GOOS, runtime.GOARCH))

	var availablePacks []AvailablePack
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}

		var bundleURL, metadataURL string
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
			Manifest:   *manifest,
			BundleURL:  bundleURL,
			BundleSize: bundleSize,
			Installed:  IsInstalled(manifest.ID),
			TagName:    release.TagName,
		})
	}

	return availablePacks, nil
}

func fetchMetadata(metadataURL string) (*python.Manifest, error) {
	httpClient := &http.Client{Timeout: 20 * time.Second}
	metadataResponse, err := httpClient.Get(metadataURL)
	if err != nil {
		return nil, err
	}
	defer metadataResponse.Body.Close()
	if metadataResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata %s: status %d", metadataURL, metadataResponse.StatusCode)
	}
	var manifest python.Manifest
	if err := json.NewDecoder(metadataResponse.Body).Decode(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}
