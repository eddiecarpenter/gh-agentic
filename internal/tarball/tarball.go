// Package tarball provides utilities for downloading and extracting GitHub
// release tarballs. It is used by doctor repair, sync, and bootstrap to
// restore template files from a specific versioned release.
package tarball

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FetchFunc downloads a tarball for the given repo and version tag.
// Returns the raw gzipped tarball bytes.
// The default implementation uses HTTPS:
//
//	https://github.com/{repo}/archive/refs/tags/{tag}.tar.gz
type FetchFunc func(repo, version string) (io.ReadCloser, error)

// DefaultFetch downloads a tarball via gh api, streaming the response body.
// This leverages the user's gh authentication automatically.
var DefaultFetch FetchFunc = defaultFetch

func defaultFetch(repo, version string) (io.ReadCloser, error) {
	url := fmt.Sprintf("https://github.com/%s/archive/refs/tags/%s.tar.gz", repo, version)
	cmd := newCommand("gh", "api", url)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting download: %w", err)
	}
	return &cmdReadCloser{ReadCloser: stdout, cmd: cmd}, nil
}

// cmdReadCloser wraps an io.ReadCloser from a command's stdout pipe and
// waits for the command to finish on Close.
type cmdReadCloser struct {
	io.ReadCloser
	cmd command
}

func (c *cmdReadCloser) Close() error {
	_ = c.ReadCloser.Close()
	return c.cmd.Wait()
}

// command abstracts exec.Cmd so tests can inject fakes.
type command interface {
	StdoutPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
}

// newCommand creates a real exec.Cmd. Overridden in tests.
var newCommand = newRealCommand

// ReadTemplateConfig reads TEMPLATE_SOURCE and TEMPLATE_VERSION from the
// given repo root directory. Returns an error if either file is missing
// or empty.
func ReadTemplateConfig(root string) (repo, version string, err error) {
	sourceData, err := os.ReadFile(filepath.Join(root, "TEMPLATE_SOURCE"))
	if err != nil {
		return "", "", fmt.Errorf("TEMPLATE_SOURCE missing: %w", err)
	}
	repo = strings.TrimSpace(string(sourceData))
	if repo == "" {
		return "", "", fmt.Errorf("TEMPLATE_SOURCE is empty")
	}

	versionData, err := os.ReadFile(filepath.Join(root, "TEMPLATE_VERSION"))
	if err != nil {
		return "", "", fmt.Errorf("TEMPLATE_VERSION missing: %w", err)
	}
	version = strings.TrimSpace(string(versionData))
	if version == "" {
		return "", "", fmt.Errorf("TEMPLATE_VERSION is empty")
	}

	return repo, version, nil
}

// ExtractFromTemplate downloads the tarball for the given repo and version,
// then extracts only files matching the given path prefixes into destRoot.
// The extraction is atomic: files are first written to a temporary directory,
// then moved to destRoot only on success.
//
// pathPrefixes are relative paths within the archive (after stripping the
// top-level GitHub prefix directory). For example: "base/skills/",
// ".goose/recipes/", ".github/workflows/".
func ExtractFromTemplate(templateRepo, version, destRoot string, pathPrefixes []string, fetch FetchFunc) error {
	if templateRepo == "" {
		return fmt.Errorf("template repo is empty — cannot fetch tarball")
	}
	if version == "" {
		return fmt.Errorf("template version is empty — cannot fetch tarball")
	}
	if fetch == nil {
		fetch = DefaultFetch
	}

	// Download the tarball.
	body, err := fetch(templateRepo, version)
	if err != nil {
		return fmt.Errorf("fetching tarball: %w", err)
	}
	defer body.Close()

	// Extract to a temporary directory for atomicity.
	tmpDir, err := os.MkdirTemp("", "tarball-extract-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir) // Clean up on any failure path.

	if err := extractTarGz(body, tmpDir, pathPrefixes); err != nil {
		return fmt.Errorf("extracting tarball: %w", err)
	}

	// Move extracted files from tmpDir to destRoot.
	if err := copyTree(tmpDir, destRoot); err != nil {
		return fmt.Errorf("writing extracted files: %w", err)
	}

	return nil
}

// extractTarGz reads a gzipped tar stream and extracts files whose paths
// (after stripping the top-level archive prefix) match any of the given
// path prefixes. Files are written under destDir.
func extractTarGz(r io.Reader, destDir string, pathPrefixes []string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("invalid gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		// Strip the top-level prefix directory (e.g. "repo-name-tag/").
		relPath := stripTopDir(hdr.Name)
		if relPath == "" {
			continue
		}

		// Check if this file matches any requested prefix.
		if !matchesAnyPrefix(relPath, pathPrefixes) {
			continue
		}

		destPath := filepath.Join(destDir, relPath)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", relPath, err)
			}
		case tar.TypeReg:
			if err := writeFile(destPath, tr, hdr.FileInfo().Mode()); err != nil {
				return fmt.Errorf("writing %s: %w", relPath, err)
			}
		}
	}
	return nil
}

// stripTopDir removes the first path component from a tar entry name.
// GitHub tarballs always have a top-level directory like "repo-tag/".
func stripTopDir(name string) string {
	// Normalise to forward slashes (tar standard).
	name = filepath.ToSlash(name)
	idx := strings.Index(name, "/")
	if idx < 0 {
		return ""
	}
	return name[idx+1:]
}

// matchesAnyPrefix returns true if relPath starts with any of the given prefixes.
// If prefixes is nil or empty, all paths match (extract everything).
func matchesAnyPrefix(relPath string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, p := range prefixes {
		if strings.HasPrefix(relPath, p) {
			return true
		}
	}
	return false
}

// writeFile creates parent directories and writes content to path with the given mode.
func writeFile(path string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

// copyTree recursively copies all files from src to dst, creating directories
// as needed. Existing files in dst are overwritten.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		destPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, data, info.Mode())
	})
}
