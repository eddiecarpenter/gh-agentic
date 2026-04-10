package sync

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfigYML writes a minimal .ai/config.yml into root.
func writeConfigYML(t *testing.T, root, template, version string) {
	t.Helper()
	dir := filepath.Join(root, ".ai")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("setup: mkdir .ai: %v", err)
	}
	content := "template: " + template + "\nversion: " + version + "\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatalf("setup: write config.yml: %v", err)
	}
}

func TestReadTemplateSource(t *testing.T) {
	t.Run("reads from config.yml", func(t *testing.T) {
		root := t.TempDir()
		writeConfigYML(t, root, "eddiecarpenter/ai-native-delivery", "v1.0.0")
		got, err := ReadTemplateSource(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "eddiecarpenter/ai-native-delivery" {
			t.Errorf("got %q, want %q", got, "eddiecarpenter/ai-native-delivery")
		}
	})

	t.Run("config.yml takes precedence over TEMPLATE_SOURCE", func(t *testing.T) {
		root := t.TempDir()
		writeConfigYML(t, root, "eddiecarpenter/ai-native-delivery", "v1.0.0")
		if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte("old/repo"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		got, err := ReadTemplateSource(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "eddiecarpenter/ai-native-delivery" {
			t.Errorf("config.yml should win: got %q", got)
		}
	})

	// TODO(deprecated): remove fallback tests in next major version when TEMPLATE_SOURCE is dropped.
	t.Run("fallback: reads from TEMPLATE_SOURCE when config.yml absent", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte("eddiecarpenter/ai-native-delivery"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		got, err := ReadTemplateSource(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "eddiecarpenter/ai-native-delivery" {
			t.Errorf("got %q, want %q", got, "eddiecarpenter/ai-native-delivery")
		}
	})

	t.Run("error when neither config.yml nor TEMPLATE_SOURCE present", func(t *testing.T) {
		root := t.TempDir()
		_, err := ReadTemplateSource(root)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("error when config.yml present but template field empty and TEMPLATE_SOURCE absent", func(t *testing.T) {
		root := t.TempDir()
		writeConfigYML(t, root, "", "v1.0.0")
		_, err := ReadTemplateSource(root)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestReadTemplateVersion(t *testing.T) {
	t.Run("reads from config.yml", func(t *testing.T) {
		root := t.TempDir()
		writeConfigYML(t, root, "eddiecarpenter/ai-native-delivery", "v1.5.0")
		got, err := ReadTemplateVersion(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "v1.5.0" {
			t.Errorf("got %q, want %q", got, "v1.5.0")
		}
	})

	t.Run("config.yml takes precedence over TEMPLATE_VERSION", func(t *testing.T) {
		root := t.TempDir()
		writeConfigYML(t, root, "eddiecarpenter/ai-native-delivery", "v1.5.0")
		if err := os.WriteFile(filepath.Join(root, "TEMPLATE_VERSION"), []byte("v0.0.1"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		got, err := ReadTemplateVersion(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "v1.5.0" {
			t.Errorf("config.yml should win: got %q", got)
		}
	})

	// TODO(deprecated): remove fallback tests in next major version when TEMPLATE_VERSION is dropped.
	t.Run("fallback: reads from TEMPLATE_VERSION when config.yml absent", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "TEMPLATE_VERSION"), []byte("v0.1.0"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		got, err := ReadTemplateVersion(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "v0.1.0" {
			t.Errorf("got %q, want %q", got, "v0.1.0")
		}
	})

	t.Run("error when neither config.yml nor TEMPLATE_VERSION present", func(t *testing.T) {
		root := t.TempDir()
		_, err := ReadTemplateVersion(root)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("error when config.yml present but version field empty and TEMPLATE_VERSION absent", func(t *testing.T) {
		root := t.TempDir()
		writeConfigYML(t, root, "eddiecarpenter/ai-native-delivery", "")
		_, err := ReadTemplateVersion(root)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// strPtr returns a pointer to the given string. Helper for table-driven tests.
func strPtr(s string) *string {
	return &s
}
