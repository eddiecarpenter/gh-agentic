package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// credentialsFilePath is the relative path from home to the Claude credentials file.
const credentialsFilePath = ".claude/.credentials.json"

// ReadClaudeCredentials reads Claude credentials from the platform-appropriate
// store. On macOS it tries the file first, then falls back to the macOS Keychain.
// On Linux/Windows, it reads from the file only.
func ReadClaudeCredentials(run RunCommandFunc, readFile func(string) ([]byte, error), homeDir func() (string, error)) ([]byte, error) {
	home, err := homeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	credPath := filepath.Join(home, credentialsFilePath)
	data, err := readFile(credPath)
	if err == nil {
		return data, nil
	}

	// Fall back to macOS Keychain.
	out, keychainErr := run("security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
	out = strings.TrimSpace(out)
	if keychainErr != nil || out == "" {
		return nil, fmt.Errorf("credentials not found at %s and macOS Keychain", credPath)
	}
	return []byte(out), nil
}

// ReadClaudeCredentialsDefault is a convenience wrapper that uses os.ReadFile
// and os.UserHomeDir as the file and home directory implementations.
func ReadClaudeCredentialsDefault(run RunCommandFunc) ([]byte, error) {
	return ReadClaudeCredentials(run, os.ReadFile, os.UserHomeDir)
}
