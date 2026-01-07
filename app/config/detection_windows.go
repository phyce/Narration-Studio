//go:build windows

package config

import (
	"golang.org/x/sys/windows/registry"
)

func getInstallDirectory() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `Software\Narration Studio`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	installPath, _, err := key.GetStringValue("InstallPath")
	if err != nil {
		return "", err
	}

	return installPath, nil
}
