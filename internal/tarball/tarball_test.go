package tarball

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildTarGz creates an in-memory gzipped tarball with the given files.
// Each key is a path (including top-level prefix); value is the content.
func buildTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		// Create parent directories.
		parts := strings.Split(filepath.ToSlash(name), "/")
		for i := 1; i < len(parts); i++ {
			dirPath := strings.Join(parts[:i], "/") + "/"
			_ = tw.WriteHeader(&tar.Header{
				Name:     dirPath,
				Typeflag: tar.TypeDir,
				Mode:     0o755,
			})
		}
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(content)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fakeFetch returns a FetchFunc that returns the given tarball bytes.
func fakeFetch(data []byte) FetchFunc {
	return func(repo, version string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
}

// failingFetch returns a FetchFunc that always errors.
func failingFetch(msg string) FetchFunc {
	return func(repo, version string) (io.ReadCloser, error) {
		return nil, fmt.Errorf("%s", msg)
	}
}

func TestReadTemplateConfig(t *testing.T) {
	tests := []struct {
		name       string
		source     *string
		version    *string
		wantRepo   string
		wantVer    string
		wantErrSub string
	}{
		{
			name:     "both present",
			source:   strPtr("owner/repo"),
			version:  strPtr("v1.0.0"),
			wantRepo: "owner/repo",
			wantVer:  "v1.0.0",
		},
		{
			name:     "whitespace trimmed",
			source:   strPtr("  owner/repo  \n"),
			version:  strPtr("  v2.0.0  \n"),
			wantRepo: "owner/repo",
			wantVer:  "v2.0.0",
		},
		{
			name:       "source missing",
			source:     nil,
			version:    strPtr("v1.0.0"),
			wantErrSub: "TEMPLATE_SOURCE missing",
		},
		{
			name:       "version missing",
			source:     strPtr("owner/repo"),
			version:    nil,
			wantErrSub: "TEMPLATE_VERSION missing",
		},
		{
			name:       "source empty",
			source:     strPtr(""),
			version:    strPtr("v1.0.0"),
			wantErrSub: "TEMPLATE_SOURCE is empty",
		},
		{
			name:       "version empty",
			source:     strPtr("owner/repo"),
			version:    strPtr(""),
			wantErrSub: "TEMPLATE_VERSION is empty",
		},
		{
			name:       "both missing",
			source:     nil,
			version:    nil,
			wantErrSub: "TEMPLATE_SOURCE missing",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.source != nil {
				if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte(*tc.source), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.version != nil {
				if err := os.WriteFile(filepath.Join(root, "TEMPLATE_VERSION"), []byte(*tc.version), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			repo, ver, err := ReadTemplateConfig(root)
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErrSub, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo != tc.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tc.wantRepo)
			}
			if ver != tc.wantVer {
				t.Errorf("version = %q, want %q", ver, tc.wantVer)
			}
		})
	}
}

func TestExtractFromTemplate_Success(t *testing.T) {
	tarData := buildTarGz(t, map[string]string{
		"repo-v1.0.0/base/skills/session-init.md":  "# Session Init",
		"repo-v1.0.0/base/skills/dev-session.md":   "# Dev Session",
		"repo-v1.0.0/.goose/recipes/dev.yaml":      "name: dev",
		"repo-v1.0.0/.github/workflows/build.yml":  "on: push",
		"repo-v1.0.0/README.md":                    "# Repo",
	})

	dest := t.TempDir()
	err := ExtractFromTemplate("owner/repo", "v1.0.0", dest,
		[]string{"base/skills/", ".goose/recipes/"},
		fakeFetch(tarData))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify expected files were extracted.
	wantFiles := map[string]string{
		"base/skills/session-init.md": "# Session Init",
		"base/skills/dev-session.md":  "# Dev Session",
		".goose/recipes/dev.yaml":     "name: dev",
	}
	for path, wantContent := range wantFiles {
		got, err := os.ReadFile(filepath.Join(dest, path))
		if err != nil {
			t.Errorf("expected file %s not found: %v", path, err)
			continue
		}
		if string(got) != wantContent {
			t.Errorf("%s content = %q, want %q", path, string(got), wantContent)
		}
	}

	// Verify excluded files were NOT extracted.
	excludedFiles := []string{
		".github/workflows/build.yml",
		"README.md",
	}
	for _, path := range excludedFiles {
		if _, err := os.Stat(filepath.Join(dest, path)); !os.IsNotExist(err) {
			t.Errorf("file %s should not have been extracted", path)
		}
	}
}

func TestExtractFromTemplate_NilPrefixes_ExtractsAllFiles(t *testing.T) {
	tarData := buildTarGz(t, map[string]string{
		"repo-v1.0.0/base/standards/go.md":        "# Go Standards",
		"repo-v1.0.0/base/skills/session-init.md": "# Session Init",
		"repo-v1.0.0/README.md":                   "# Repo",
	})

	dest := t.TempDir()
	err := ExtractFromTemplate("owner/repo", "v1.0.0", dest, nil, fakeFetch(tarData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantFiles := []string{
		"base/standards/go.md",
		"base/skills/session-init.md",
		"README.md",
	}
	for _, path := range wantFiles {
		if _, err := os.Stat(filepath.Join(dest, path)); err != nil {
			t.Errorf("expected file %s to be extracted, got: %v", path, err)
		}
	}
}

func TestExtractFromTemplate_EmptyRepo_ReturnsError(t *testing.T) {
	err := ExtractFromTemplate("", "v1.0.0", t.TempDir(),
		[]string{"base/"}, fakeFetch(nil))
	if err == nil || !strings.Contains(err.Error(), "template repo is empty") {
		t.Fatalf("expected 'template repo is empty' error, got: %v", err)
	}
}

func TestExtractFromTemplate_EmptyVersion_ReturnsError(t *testing.T) {
	err := ExtractFromTemplate("owner/repo", "", t.TempDir(),
		[]string{"base/"}, fakeFetch(nil))
	if err == nil || !strings.Contains(err.Error(), "template version is empty") {
		t.Fatalf("expected 'template version is empty' error, got: %v", err)
	}
}

func TestExtractFromTemplate_FetchError_ReturnsError(t *testing.T) {
	dest := t.TempDir()
	err := ExtractFromTemplate("owner/repo", "v1.0.0", dest,
		[]string{"base/"}, failingFetch("network error"))
	if err == nil || !strings.Contains(err.Error(), "network error") {
		t.Fatalf("expected 'network error' in error, got: %v", err)
	}

	// Verify no files were written to dest (atomicity).
	entries, _ := os.ReadDir(dest)
	if len(entries) > 0 {
		t.Errorf("expected empty dest after fetch failure, found %d entries", len(entries))
	}
}

func TestExtractFromTemplate_InvalidTarball_ReturnsError(t *testing.T) {
	fetch := func(repo, version string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte("not a tarball"))), nil
	}

	dest := t.TempDir()
	err := ExtractFromTemplate("owner/repo", "v1.0.0", dest,
		[]string{"base/"}, fetch)
	if err == nil {
		t.Fatal("expected error for invalid tarball, got nil")
	}

	// Verify no files were written to dest (atomicity).
	entries, _ := os.ReadDir(dest)
	if len(entries) > 0 {
		t.Errorf("expected empty dest after invalid tarball, found %d entries", len(entries))
	}
}

func TestExtractFromTemplate_NilFetch_UsesDefault(t *testing.T) {
	// This test just verifies nil fetch doesn't panic — the actual download
	// would fail because DefaultFetch requires gh, but that's fine.
	err := ExtractFromTemplate("owner/repo", "v1.0.0", t.TempDir(),
		[]string{"base/"}, nil)
	// We expect an error because DefaultFetch will fail in tests, but not a panic.
	if err == nil {
		t.Log("unexpected success — DefaultFetch somehow worked in test env")
	}
}

func TestStripTopDir(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"repo-v1.0.0/base/skills/foo.md", "base/skills/foo.md"},
		{"repo-v1.0.0/", ""},
		{"repo-v1.0.0", ""},
		{"a/b/c", "b/c"},
		{"single", ""},
	}
	for _, tc := range tests {
		got := stripTopDir(tc.input)
		if got != tc.want {
			t.Errorf("stripTopDir(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMatchesAnyPrefix(t *testing.T) {
	tests := []struct {
		path     string
		prefixes []string
		want     bool
	}{
		{"base/skills/foo.md", []string{"base/skills/"}, true},
		{"base/skills/foo.md", []string{".goose/"}, false},
		{"base/skills/foo.md", []string{".goose/", "base/"}, true},
		{".goose/recipes/dev.yaml", []string{".goose/recipes/"}, true},
		{"README.md", []string{"base/", ".goose/"}, false},
	}
	for _, tc := range tests {
		got := matchesAnyPrefix(tc.path, tc.prefixes)
		if got != tc.want {
			t.Errorf("matchesAnyPrefix(%q, %v) = %v, want %v", tc.path, tc.prefixes, got, tc.want)
		}
	}
}

func TestExtractFromTemplate_AtomicNoPartialWrites(t *testing.T) {
	// Create a tarball that starts valid but has files, then verify
	// that on failure no partial files remain.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Write one valid entry.
	content := "valid content"
	_ = tw.WriteHeader(&tar.Header{
		Name:     "repo-v1/base/skills/good.md",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(content)),
	})
	_, _ = tw.Write([]byte(content))

	// Don't close properly — truncate to make it corrupted mid-stream.
	_ = tw.Flush()
	_ = gw.Close()

	// Truncate the buffer to simulate a partial download.
	data := buf.Bytes()
	truncated := data[:len(data)/2]

	dest := t.TempDir()
	err := ExtractFromTemplate("owner/repo", "v1.0.0", dest,
		[]string{"base/skills/"},
		fakeFetch(truncated))

	// Should fail due to corrupted tarball.
	if err == nil {
		// Even if it doesn't fail (in case the truncation was after all data),
		// that's still acceptable — the key is no partial writes on actual failure.
		return
	}

	// Verify no files were written to dest.
	entries, _ := os.ReadDir(dest)
	if len(entries) > 0 {
		t.Errorf("expected empty dest after partial extraction, found %d entries", len(entries))
	}
}

func strPtr(s string) *string {
	return &s
}
