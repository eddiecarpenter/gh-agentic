package project

import (
	"os"
	"path/filepath"
)

// IsFrameworkSource reports whether the repository rooted at root is the
// gh-agentic framework source itself, rather than a consumer.
//
// The signal is structural: in a consumer repo, .ai/ is a directory
// populated by `gh agentic mount` (via `git clone` under the hood) and
// gitignored. In the framework repo, .ai is a committed symlink pointing
// at "." so that any recipe or script that references .ai/.goose/…,
// .ai/skills/, .ai/standards/, etc. resolves identically in both contexts.
//
// Detection: .ai is a symlink → framework source. This is the single
// signal used by every CLI command that must refuse or adapt its
// behaviour on the framework repo (see RULEBOOK / feature #619).
//
// Returns false if .ai does not exist, is a regular directory, or any
// other file type. The check is cheap (one lstat) and is safe to call
// at the entry of any command.
func IsFrameworkSource(root string) bool {
	info, err := os.Lstat(filepath.Join(root, ".ai"))
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
