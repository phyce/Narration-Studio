package profile

import (
	"nstudio/app/common/util"
)

func NewProfile(id, name string) *Profile {
	if name == "" {
		name = id
	}

	now := util.GetCurrentTimestamp()

	return &Profile{
		ID:        id,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
		Voices:    make(map[string]*util.CharacterVoice),
	}
}

func (profile *Profile) GetMetadata() ProfileMetadata {
	profile.mutex.RLock()
	voiceCount := len(profile.Voices)
	profile.mutex.RUnlock()

	return ProfileMetadata{
		ID:          profile.ID,
		Name:        profile.Name,
		Description: profile.Description,
		CreatedAt:   profile.CreatedAt,
		UpdatedAt:   profile.UpdatedAt,
		VoiceCount:  voiceCount,
	}
}

func (profile *Profile) GetVoice(character string) (*util.CharacterVoice, bool) {
	profile.mutex.RLock()
	voice, exists := profile.Voices[character]
	profile.mutex.RUnlock()
	return voice, exists
}

func (profile *Profile) SetVoice(character string, voice *util.CharacterVoice) {
	profile.mutex.Lock()
	profile.Voices[character] = voice
	profile.UpdatedAt = util.GetCurrentTimestamp()
	profile.mutex.Unlock()
}

func (profile *Profile) RemoveVoice(character string) bool {
	profile.mutex.Lock()
	defer profile.mutex.Unlock()

	if _, exists := profile.Voices[character]; exists {
		delete(profile.Voices, character)
		profile.UpdatedAt = util.GetCurrentTimestamp()
		return true
	}
	return false
}

func (profile *Profile) GetCharacters() []string {
	profile.mutex.RLock()
	characters := make([]string, 0, len(profile.Voices))
	for character := range profile.Voices {
		characters = append(characters, character)
	}
	profile.mutex.RUnlock()
	return characters
}

func (profile *Profile) GetSettings() *ProfileSettings {
	return profile.Settings
}

func (profile *Profile) SetSettings(settings *ProfileSettings) {
	profile.mutex.Lock()
	profile.Settings = settings
	profile.UpdatedAt = util.GetCurrentTimestamp()
	profile.mutex.Unlock()
}

func (profile *Profile) GetModelToggles() map[string]bool {
	profile.mutex.RLock()
	defer profile.mutex.RUnlock()
	if profile.Settings == nil {
		return nil
	}
	return profile.Settings.ModelToggles
}
