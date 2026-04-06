package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTemplateSource(t *testing.T) {
	tests := []struct {
		name      string
		content   *string // nil means don't create the file
		want      string
		wantError bool
	}{
		{
			name:    "valid content",
			content: strPtr("eddiecarpenter/ai-native-delivery"),
			want:    "eddiecarpenter/ai-native-delivery",
		},
		{
			name:    "content with whitespace",
			content: strPtr("  eddiecarpenter/ai-native-delivery  \n"),
			want:    "eddiecarpenter/ai-native-delivery",
		},
		{
			name:      "missing file",
			content:   nil,
			wantError: true,
		},
		{
			name:      "empty file",
			content:   strPtr(""),
			wantError: true,
		},
		{
			name:      "whitespace only",
			content:   strPtr("   \n  "),
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()

			if tc.content != nil {
				err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte(*tc.content), 0o644)
				if err != nil {
					t.Fatalf("setup: %v", err)
				}
			}

			got, err := ReadTemplateSource(root)

			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadTemplateVersion(t *testing.T) {
	tests := []struct {
		name      string
		content   *string // nil means don't create the file
		want      string
		wantError bool
	}{
		{
			name:    "valid content",
			content: strPtr("v0.1.0"),
			want:    "v0.1.0",
		},
		{
			name:    "content with whitespace",
			content: strPtr("  v0.2.0  \n"),
			want:    "v0.2.0",
		},
		{
			name:      "missing file",
			content:   nil,
			wantError: true,
		},
		{
			name:      "empty file",
			content:   strPtr(""),
			wantError: true,
		},
		{
			name:      "whitespace only",
			content:   strPtr("  \t\n"),
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()

			if tc.content != nil {
				err := os.WriteFile(filepath.Join(root, "TEMPLATE_VERSION"), []byte(*tc.content), 0o644)
				if err != nil {
					t.Fatalf("setup: %v", err)
				}
			}

			got, err := ReadTemplateVersion(root)

			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// strPtr returns a pointer to the given string. Helper for table-driven tests.
func strPtr(s string) *string {
	return &s
}
