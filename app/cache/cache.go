package cache

import (
	"encoding/json"
	"fmt"
	"nstudio/app/common/util"
	"nstudio/app/tts/profile"
	"os"
	"path/filepath"
	"sync"

	"nstudio/app/common/response"
	"nstudio/app/config"
)

var (
	globalManager *CacheManager
	mutex         sync.RWMutex
)

func Initialize() error {
	mutex.Lock()
	defer mutex.Unlock()

	settings := config.GetSettings()

	if !settings.AudioCache.Enabled {
		if globalManager != nil {
			globalManager.enabled = false
			response.LogInfo("Cache disabled")
		}
		return nil
	}

	err, cacheDir := util.ExpandPath(settings.AudioCache.Location)
	if err != nil {
		return response.Warn("failed to expand cache directory path: %v", err)
	}
	if cacheDir == "" {
		response.NewWarn("Cache directory is not configured")
		return nil
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return response.Warn("failed to create cache directory: %v", err)
	}

	if globalManager == nil {
		globalManager = &CacheManager{
			enabled:  true,
			cacheDir: cacheDir,
			profiles: make(map[string]*ProfileCache),
		}

		response.LogInfo(fmt.Sprintf("Cache manager initialized with directory: %s\n", cacheDir))
	} else {
		globalManager.enabled = true
		globalManager.cacheDir = cacheDir
		response.LogInfo(fmt.Sprintf("Cache manager updated with directory: %s\n", cacheDir))
	}

	return nil
}

func GetManager() *CacheManager {
	if globalManager == nil {
		Initialize()
	}
	return globalManager
}

func (cacheManager *CacheManager) IsEnabled() bool {
	if cacheManager == nil {
		return false
	}
	return cacheManager.enabled
}

// <editor-fold desc="Profile Cache">
func (cacheManager *CacheManager) loadProfileCache(profileID string) (*ProfileCache, error) {
	cacheManager.mutex.RLock()
	if cache, exists := cacheManager.profiles[profileID]; exists {
		cacheManager.mutex.RUnlock()
		return cache, nil
	}
	cacheManager.mutex.RUnlock()

	cacheFile := cacheManager.getCharacterCacheFile(profileID)
	cache := &ProfileCache{
		Characters: make(map[string]*CharacterCache),
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, response.Err(err)
		}
	} else {
		if err := json.Unmarshal(data, &cache.Characters); err != nil {
			return nil, response.Err(fmt.Errorf("Failed to parse cache file: %v", err))
		}
	}

	cacheManager.mutex.Lock()
	cacheManager.profiles[profileID] = cache
	cacheManager.mutex.Unlock()
	return cache, nil
}

func (cacheManager *CacheManager) saveProfileCache(profileID string, profileCache *ProfileCache) error {
	cacheDirectory := cacheManager.getProfileCacheDirectory(profileID)
	if err := os.MkdirAll(cacheDirectory, 0755); err != nil {
		return response.Err(err)
	}

	profileCache.mutex.RLock()
	data, err := json.MarshalIndent(profileCache.Characters, "", "  ")
	profileCache.mutex.RUnlock()

	if err != nil {
		return response.Err(err)
	}

	cacheFile := cacheManager.getCharacterCacheFile(profileID)
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return response.Err(err)
	}

	return nil
}

func (cacheManager *CacheManager) ClearProfileCache(profileID string) error {
	if !cacheManager.enabled {
		return nil
	}

	cacheManager.mutex.Lock()
	delete(cacheManager.profiles, profileID)
	cacheManager.mutex.Unlock()

	cacheDirectory := cacheManager.getProfileCacheDirectory(profileID)
	if err := os.RemoveAll(cacheDirectory); err != nil && !os.IsNotExist(err) {
		return response.Err(err)
	}

	return nil
}

func (cacheManager *CacheManager) getProfileCacheDirectory(profileID string) string {
	return filepath.Join(cacheManager.cacheDir, "profiles", profileID)
}

func (cacheManager *CacheManager) getProfileAudioDir(profileID string) string {
	return filepath.Join(cacheManager.getProfileCacheDirectory(profileID), "audio")
}

//</editor-fold>

// <editor-fold desc="Character Cache">
func (cacheManager *CacheManager) GetCachedAudio(profileID, character, text string) ([]byte, bool) {
	profileCache, err := cacheManager.loadProfileCache(profileID)
	if err != nil {
		response.Warn("Failed to load profile cache: %v\n", err)
		return nil, false
	}

	profileCache.mutex.RLock()

	charCache, exists := profileCache.Characters[character]
	profileCache.mutex.RUnlock()

	if !exists {
		return nil, false
	}
	profileManager := profile.GetManager()
	voice, err := profileManager.GetOrAllocateVoice(profileID, character)
	if err != nil {
		response.Warn("Failed to get voice: %v\n", err)
		return nil, false
	}

	if charCache.Voice != voice.Key() {
		response.NewWarn(fmt.Sprintf("Voice mismatch for '%s'. Expected: %s, Got: %s\n", character, voice.Key(), charCache.Voice))
		return nil, false
	}

	textHash := util.HashText(text)[:8]
	filename, exists := charCache.Lines[textHash]
	if !exists {
		return nil, false
	}

	audioPath := filepath.Join(cacheManager.getCharacterAudioDir(profileID, character), filename)
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		response.Warn("Failed to read audio file: %v\n", err)
		return nil, false
	}

	response.LogInfo(fmt.Sprintf("Get Cached Audio: Successfully loaded %d bytes from cache for '%s'\n", len(audioData), character))
	return audioData, true
}

func (cacheManager *CacheManager) CacheAudio(profileID, characterName, text, voiceKey string, rawAudio []byte) error {
	profileCache, err := cacheManager.loadProfileCache(profileID)
	if err != nil {
		return response.Err(err)
	}

	profileCache.mutex.Lock()
	characterCache, exists := profileCache.Characters[characterName]
	if !exists {
		characterCache = &CharacterCache{
			Voice: voiceKey,
			Lines: make(map[string]string),
		}
		profileCache.Characters[characterName] = characterCache
	} else if characterCache.Voice != voiceKey {
		characterCache.Voice = voiceKey
		characterCache.Lines = make(map[string]string)
	}
	profileCache.mutex.Unlock()

	textHash := util.HashText(text)[:8]
	sanitizedFilename := util.SanitizeFilename(text)
	if sanitizedFilename == "" {
		sanitizedFilename = "audio"
	}

	filename := fmt.Sprintf("%s_%s.wav", sanitizedFilename, textHash)

	audioDir := cacheManager.getCharacterAudioDir(profileID, characterName)
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		return response.Err(err)
	}

	audioPath := filepath.Join(audioDir, filename)
	outputFile, err := os.Create(audioPath)
	if err != nil {
		fmt.Printf("[Cache] CacheAudio: Error creating file: %v\n", err)
		return response.Err(err)
	}
	defer outputFile.Close()

	_, err = outputFile.Write(rawAudio)
	if err != nil {
		return response.Err(err)
	}

	profileCache.mutex.Lock()
	characterCache.Lines[textHash] = filename
	profileCache.mutex.Unlock()

	if err := cacheManager.saveProfileCache(profileID, profileCache); err != nil {
		return response.Alert("CacheAudio: Failed to save cache metadata: %v\n", err)
	}

	return nil
}

func (cacheManager *CacheManager) ClearCharacterCache(profileID, character string) error {
	if !cacheManager.enabled {
		return nil
	}

	profileCache, err := cacheManager.loadProfileCache(profileID)
	if err != nil {
		return response.Err(err)
	}

	profileCache.mutex.Lock()
	_, characterIsCached := profileCache.Characters[character]
	if !characterIsCached {
		profileCache.mutex.Unlock()
		return nil
	}
	profileCache.mutex.Unlock()

	characterDir := cacheManager.getCharacterAudioDir(profileID, character)
	if err := os.RemoveAll(characterDir); err != nil && !os.IsNotExist(err) {
		response.Warn("failed to remove character directory: %v", err)
	}

	profileCache.mutex.Lock()
	delete(profileCache.Characters, character)
	profileCache.mutex.Unlock()

	return cacheManager.saveProfileCache(profileID, profileCache)
}

func (cacheManager *CacheManager) getCharacterAudioDir(profileID, character string) string {
	return filepath.Join(cacheManager.getProfileCacheDirectory(profileID), character)
}

func (cacheManager *CacheManager) getCharacterCacheFile(profileID string) string {
	return filepath.Join(cacheManager.getProfileCacheDirectory(profileID), "characters.json")
}

//</editor-fold>
