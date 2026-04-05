package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadAgentUser reads the AGENT_USER file from the repo root and returns the
// configured agent username. Returns empty string (no error) if the file does
// not exist. Trims whitespace from the value.
func ReadAgentUser(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "AGENT_USER"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
