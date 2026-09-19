package skritter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadToken shares yinyang's environment variable, with an optional local file.
// The credential is never serialized into the application's TOML configuration.
func LoadToken() (string, error) {
	if token := strings.TrimSpace(os.Getenv("SKRITTER_TOKEN")); token != "" {
		return token, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(home, ".polybius", "skritter-token"))
	if os.IsNotExist(err) {
		return "", fmt.Errorf("Skritter is not connected; set SKRITTER_TOKEN or save ~/.polybius/skritter-token")
	}
	if err != nil {
		return "", fmt.Errorf("cannot read Skritter token file")
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("Skritter token file is empty")
	}
	return token, nil
}
