//go:build !windows

package platform

import (
	"os"
	"path/filepath"
)

func LineSeparator() string {
	return "\n"
}

// GetAssetLocation searches for `file` in the env dir, the executable dir, and certain locations
func GetAssetLocation(file string) string {
	assetPath := NewEnvFlag(AssetLocation).GetValue(getExecutableDir)
	defPath := filepath.Join(assetPath, file)
	for _, p := range []string{
		defPath,
		"/usr/local/share/xray/" + file,
		"/usr/share/xray/" + file,
		"/opt/share/xray/" + file,
	} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			continue
		}

		// asset found
		return p
	}

	// asset not found, let the caller throw out the error
	return defPath
}

// GetCertLocation searches for `file` in the env dir and the executable dir
func GetCertLocation(file string) string {
	certPath := NewEnvFlag(CertLocation).GetValue(getExecutableDir)
	return filepath.Join(certPath, file)
}
