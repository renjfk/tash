package data

import (
	"os"
	"path/filepath"
)

const profileFile = "profile.md"

// Profile holds the contents of profile.md.
type Profile struct {
	Content string
}

// ReadProfile loads profile.md from the data directory.
func ReadProfile(dataDir string) (*Profile, error) {
	path := filepath.Join(dataDir, profileFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &Profile{Content: string(data)}, nil
}

// WriteProfile saves profile.md to the data directory.
func WriteProfile(dataDir string, content string) error {
	path := filepath.Join(dataDir, profileFile)
	return os.WriteFile(path, []byte(content), 0644)
}
