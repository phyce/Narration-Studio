package profile

import (
	"fmt"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/tts/engine"
	"strings"
	"sync"
)

var (
	globalManager *ProfileManager
	managerOnce   sync.Once
)

func GetManager() *ProfileManager {
	managerOnce.Do(func() {
		globalManager = &ProfileManager{
			cache: make(map[string]*Profile),
		}
	})
	return globalManager
}

func (manager *ProfileManager) GetProfile(profileID string) (*Profile, error) {
	manager.mutex.RLock()
	cached, exists := manager.cache[profileID]
	manager.mutex.RUnlock()

	if exists {
		return cached, nil
	}

	profile, err := LoadProfile(profileID)
	if err != nil {
		return nil, err
	}

	manager.mutex.Lock()
	manager.cache[profileID] = profile
	manager.mutex.Unlock()

	return profile, nil
}

func (manager *ProfileManager) GetAllProfiles() ([]ProfileMetadata, error) {
	return ListProfiles()
}

func (manager *ProfileManager) CreateProfile(id, name, description string) (*Profile, error) {
	if id == "" {
		return nil, fmt.Errorf("profile ID cannot be empty")
	}

	if ProfileExists(id) {
		return nil, fmt.Errorf("profile already exists: %s", id)
	}

	profile := NewProfile(id, name)
	profile.Description = description

	if err := SaveProfile(profile); err != nil {
		return nil, response.Err(err)
	}

	manager.mutex.Lock()
	manager.cache[id] = profile
	manager.mutex.Unlock()

	return profile, nil
}

func (manager *ProfileManager) SaveProfile(profile *Profile) error {
	if err := SaveProfile(profile); err != nil {
		return response.Err(err)
	}

	manager.mutex.Lock()
	manager.cache[profile.ID] = profile
	manager.mutex.Unlock()

	return nil
}

func (manager *ProfileManager) DeleteProfile(profileID string) error {
	if err := DeleteProfile(profileID); err != nil {
		return err
	}

	manager.mutex.Lock()
	delete(manager.cache, profileID)
	manager.mutex.Unlock()

	return nil
}

func (manager *ProfileManager) GetVoiceConfig(profileID, character string) (*util.CharacterVoice, error) {
	profile, err := manager.GetProfile(profileID)
	if err != nil {
		return nil, err
	}

	voice, exists := profile.GetVoice(character)
	if !exists {
		return nil, fmt.Errorf("character not found in profile: %s", character)
	}

	return voice, nil
}

func (manager *ProfileManager) SetVoiceConfig(profileID, character string, voice *util.CharacterVoice) error {
	profile, err := manager.GetProfile(profileID)
	if err != nil {
		return err
	}

	profile.SetVoice(character, voice)

	return manager.SaveProfile(profile)
}

func (manager *ProfileManager) RemoveVoiceConfig(profileID, character string) error {
	profile, err := manager.GetProfile(profileID)
	if err != nil {
		return err
	}

	if !profile.RemoveVoice(character) {
		return fmt.Errorf("character not found in profile: %s", character)
	}

	return manager.SaveProfile(profile)
}

func (manager *ProfileManager) GetOrAllocateVoice(profileID, character string) (*util.CharacterVoice, error) {
	// Override voices (prefixed with "::") should be resolved without saving to the profile
	if strings.HasPrefix(character, "::") {
		allocatedVoice, err := manager.AllocateVoiceForProfile(character, profileID)
		if err != nil {
			return nil, response.Err(fmt.Errorf("failed to allocate voice: %v", err))
		}
		return &allocatedVoice, nil
	}

	profile, err := manager.GetProfile(profileID)
	if err != nil {
		if !ProfileExists(profileID) {
			profile, err = manager.CreateProfile(profileID, profileID, "")
			if err != nil {
				return nil, response.Err(err)
			}
		} else {
			return nil, response.Err(err)
		}
	}

	voice, exists := profile.GetVoice(character)
	if exists && voice.Engine != "" && voice.Model != "" {
		return voice, nil
	}

	allocatedVoice, err := manager.AllocateVoiceForProfile(character, profileID)
	if err != nil {
		return nil, response.Err(fmt.Errorf("failed to allocate voice: %v", err))
	}

	profile.SetVoice(character, &allocatedVoice)
	if err := manager.SaveProfile(profile); err != nil {
		return nil, response.Err(err)
	}

	return &allocatedVoice, nil
}

func (manager *ProfileManager) AllocateVoice(name string) (util.CharacterVoice, error) {
	return manager.AllocateVoiceForProfile(name, "")
}

func (manager *ProfileManager) AllocateVoiceForProfile(name string, profileID string) (util.CharacterVoice, error) {
	if strings.HasPrefix(name, "::") {
		parts := strings.Split(name, ":")
		if len(parts) == 5 {
			characterVoice := util.CharacterVoice{
				Name:   "",
				Engine: parts[2],
				Model:  parts[3],
				Voice:  parts[4],
			}
			return characterVoice, nil
		} else {
			return util.CharacterVoice{}, response.Err(
				fmt.Errorf("Invalid line could not be processed: " + name),
			)
		}
	}

	var selectedEngine engine.Engine
	var err error

	if profileID != "" {
		profile, profileErr := manager.GetProfile(profileID)
		if profileErr == nil && profile.GetModelToggles() != nil && len(profile.GetModelToggles()) > 0 {
			selectedEngine, err = calculateProfileEngine(name, profileID)
			if err != nil {
				return util.CharacterVoice{}, response.Err(err)
			}

			model, voice, err := calculateProfileVoice(selectedEngine, name, profileID)
			if err != nil {
				return util.CharacterVoice{}, response.Err(err)
			}

			return util.CharacterVoice{
				Name:   name,
				Engine: selectedEngine.ID,
				Model:  model,
				Voice:  voice,
			}, nil
		}
	}

	selectedEngine, err = calculateEngine(name)
	if err != nil {
		return util.CharacterVoice{}, response.Err(err)
	}

	model, voice, err := calculateVoice(selectedEngine, name)
	if err != nil {
		return util.CharacterVoice{}, response.Err(err)
	}

	return util.CharacterVoice{
		Name:   name,
		Engine: selectedEngine.ID,
		Model:  model,
		Voice:  voice,
	}, nil
}

func (manager *ProfileManager) ClearCache() {
	manager.mutex.Lock()
	manager.cache = make(map[string]*Profile)
	manager.mutex.Unlock()
}

func (manager *ProfileManager) ReloadProfile(profileID string) (*Profile, error) {
	profile, err := LoadProfile(profileID)
	if err != nil {
		return nil, err
	}

	manager.mutex.Lock()
	manager.cache[profileID] = profile
	manager.mutex.Unlock()

	return profile, nil
}
