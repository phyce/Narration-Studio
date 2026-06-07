package profile

import (
	"encoding/json"
	"fmt"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"os"
	"path/filepath"
	"strings"
)

const (
	profileFileExtension = ".json"
	defaultProfileDir    = "profiles"
)

var profileDirectory string

func InitializeProfileDirectory(baseDirectory string) error {
	if baseDirectory == "" {
		currentWorkingDirectory, err := os.Getwd()
		if err != nil {
			return response.Err("failed to get working directory: %v", err)
		}
		profileDirectory = filepath.Join(currentWorkingDirectory, defaultProfileDir)
	} else {
		profileDirectory = baseDirectory
	}

	if err := os.MkdirAll(profileDirectory, 0755); err != nil {
		return response.Err("failed to create profile directory: %v", err)
	}

	profiles, err := listProfileFiles()
	if err != nil {
		return response.Err("%w", err)
	}

	if len(profiles) == 0 {
		defaultProfile := NewProfile("default", "Default Profile")
		defaultProfile.Description = "Default voice configuration profile"
		if err := SaveProfile(defaultProfile); err != nil {
			return response.Err("failed to create default profile: %v", err)
		}
	}

	return nil
}

func GetProfileDirectory() string {
	if profileDirectory == "" {
		InitializeProfileDirectory("")
	}
	return profileDirectory
}

func getProfileFilePath(profileID string) string {
	return filepath.Join(GetProfileDirectory(), profileID+profileFileExtension)
}

func LoadProfile(profileID string) (*Profile, error) {
	filePath := getProfileFilePath(profileID)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, response.Err("profile not found: %s", profileID)
		}
		return nil, response.Err("failed to read profile file: %v", err)
	}

	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, response.Err("failed to parse profile file: %v", err)
	}

	return &profile, nil
}

func SaveProfile(profile *Profile) error {
	if profile.ID == "" {
		return response.Err("profile ID cannot be empty")
	}

	if strings.ContainsAny(profile.ID, "/\\:*?\"<>|") {
		return response.Err("invalid profile ID: contains forbidden characters")
	}

	profile.UpdatedAt = util.GetCurrentTimestamp()

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return response.Err("failed to serialize profile: %v", err)
	}

	filePath := getProfileFilePath(profile.ID)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return response.Err("failed to write profile file: %v", err)
	}

	return nil
}

func DeleteProfile(profileID string) error {
	if profileID == "default" {
		return response.Err("cannot delete default profile")
	}

	filePath := getProfileFilePath(profileID)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return response.Err("profile not found: %s", profileID)
		}
		return response.Err("failed to delete profile: %v", err)
	}

	return nil
}

func listProfileFiles() ([]string, error) {
	dir := GetProfileDirectory()

	pattern := filepath.Join(dir, "*"+profileFileExtension)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, response.Err("failed to list profile files: %v", err)
	}

	return files, nil
}

func ListProfiles() ([]ProfileMetadata, error) {
	files, err := listProfileFiles()
	if err != nil {
		return nil, response.Err("%w", err)
	}

	profiles := make([]ProfileMetadata, 0, len(files))

	for _, filePath := range files {
		basename := filepath.Base(filePath)
		profileID := strings.TrimSuffix(basename, profileFileExtension)

		profile, err := LoadProfile(profileID)
		if err != nil {
			fmt.Printf("Warning: Failed to load profile %s: %v\n", profileID, err)
			continue
		}

		profiles = append(profiles, profile.GetMetadata())
	}

	return profiles, nil
}

func ProfileExists(profileID string) bool {
	filePath := getProfileFilePath(profileID)
	_, err := os.Stat(filePath)
	return err == nil
}
