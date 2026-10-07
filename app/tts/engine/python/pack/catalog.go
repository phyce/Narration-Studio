package pack

import (
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"time"

	"nstudio/app/common/response"
	"nstudio/app/tts/engine/python"
)

var CatalogURL = "https://raw.githubusercontent.com/phyce/NarrationStudio-Engines/catalog/catalog.json"

const supportedCatalogSchemaVersion = 1

type AvailablePack struct {
	Manifest    python.Manifest `json:"manifest"`
	BundleURL   string          `json:"bundleUrl"`
	BundleSize  int64           `json:"bundleSize"`
	SHA256      string          `json:"sha256,omitempty"`
	Notes       string          `json:"notes,omitempty"`
	PublishedAt string          `json:"publishedAt,omitempty"`
	Local       bool            `json:"local"`
	Compatible  bool            `json:"compatible"`
	Paused      bool            `json:"paused"`
}

type catalogFile struct {
	SchemaVersion int            `json:"schemaVersion"`
	Packs         []catalogEntry `json:"packs"`
}

type catalogEntry struct {
	Manifest    python.Manifest `json:"manifest"`
	URL         string          `json:"url"`
	Size        int64           `json:"size"`
	SHA256      string          `json:"sha256"`
	Notes       string          `json:"notes"`
	PublishedAt string          `json:"publishedAt"`
}

func FetchAvailable() ([]AvailablePack, error) {
	httpClient := &http.Client{Timeout: 20 * time.Second}
	catalogResponse, err := httpClient.Get(CatalogURL)
	if err != nil {
		return nil, response.Err("fetch engine pack catalog: %w", err)
	}
	defer catalogResponse.Body.Close()

	if catalogResponse.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if catalogResponse.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(io.LimitReader(catalogResponse.Body, 4096))
		return nil, response.Err("engine pack catalog returned %d: %s", catalogResponse.StatusCode, string(errorBody))
	}

	var catalog catalogFile
	if err := json.NewDecoder(catalogResponse.Body).Decode(&catalog); err != nil {
		return nil, response.Err("read engine pack catalog: %w", err)
	}
	if catalog.SchemaVersion != supportedCatalogSchemaVersion {
		return nil, response.Err("the engine pack catalog uses format %d; this version of Narration Studio reads format %d", catalog.SchemaVersion, supportedCatalogSchemaVersion)
	}

	var availablePacks []AvailablePack
	for _, entry := range catalog.Packs {
		if !runsOnThisPlatform(entry.Manifest) || validateIdentity(entry.Manifest) != nil {
			continue
		}
		availablePacks = append(availablePacks, AvailablePack{
			Manifest:    entry.Manifest,
			BundleURL:   entry.URL,
			BundleSize:  entry.Size,
			SHA256:      entry.SHA256,
			Notes:       entry.Notes,
			PublishedAt: entry.PublishedAt,
			Compatible:  python.SupportsProtocol(entry.Manifest.Protocol),
			Paused:      hasPausedInstall(entry.Manifest),
		})
	}
	return availablePacks, nil
}

func runsOnThisPlatform(manifest python.Manifest) bool {
	return (manifest.OS == "" || manifest.OS == runtime.GOOS) && (manifest.Arch == "" || manifest.Arch == runtime.GOARCH)
}
