//go:build !windows

package config

func getInstallDirectory() (string, error) {
	return "", nil
}
