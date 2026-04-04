package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
)

// ConfirmFunc prompts the user for a text input and returns the value.
type ConfirmFunc func(prompt string) (string, error)

// standardCLAUDEMD is the standard content for CLAUDE.md.
const standardCLAUDEMD = `# CLAUDE.md

This project uses AGENTS.md as the single source of truth for agent instructions.
All development rules, workflows, and session protocols are defined there.

@base/AGENTS.md
@AGENTS.local.md
`

// skeletonAGENTSLocalMD is the minimal skeleton for AGENTS.local.md.
const skeletonAGENTSLocalMD = `# AGENTS.local.md — Local Overrides

This file contains project-specific rules and overrides that extend or
supersede the global protocol defined in ` + "`base/AGENTS.md`" + `.

This file is never overwritten by a template sync.

---

## Template Source

<!-- TODO: Add template source -->

## Project

<!-- TODO: Add project details -->
`

// emptyREPOSMD is the standard empty REPOS.md for embedded topology.
const emptyREPOSMD = `# REPOS.md — Repository Registry

This is an embedded topology project — a single repo containing both the agentic
control plane and the project codebase. There are no separate domain or tool repos.

For organisation topology projects, this file lists all repos in the solution.

---

<!-- No entries — embedded topology -->
`

// minimalREADMEMD is a minimal README.md placeholder.
const minimalREADMEMD = `# Project

<!-- TODO: Add project name and description -->

## Setup

See ` + "`docs/PROJECT_BRIEF.md`" + ` for project context.

## Agent sessions

This repo uses the [agentic development framework](https://github.com/eddiecarpenter/agentic-development).
See ` + "`base/AGENTS.md`" + ` and ` + "`AGENTS.local.md`" + ` for session protocols.
`

// RepairCLAUDEMD restores CLAUDE.md with standard content.
func RepairCLAUDEMD(root string) CheckResult {
	path := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(path, []byte(standardCLAUDEMD), 0o644); err != nil {
		return CheckResult{
			Name:    "CLAUDE.md exists",
			Status:  Fail,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}
	return CheckResult{
		Name:   "CLAUDE.md exists",
		Status: Pass,
	}
}

// RepairAGENTSLocalMD restores AGENTS.local.md with a minimal skeleton.
func RepairAGENTSLocalMD(root string) CheckResult {
	path := filepath.Join(root, "AGENTS.local.md")
	if err := os.WriteFile(path, []byte(skeletonAGENTSLocalMD), 0o644); err != nil {
		return CheckResult{
			Name:    "AGENTS.local.md exists",
			Status:  Warning,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}
	return CheckResult{
		Name:   "AGENTS.local.md exists",
		Status: Pass,
	}
}

// RepairTEMPLATESOURCE prompts the user for the template source value and writes it.
// confirmFn is injected so tests can substitute a fake implementation.
func RepairTEMPLATESOURCE(root string, confirmFn ConfirmFunc) CheckResult {
	if confirmFn == nil {
		return CheckResult{
			Name:    "TEMPLATE_SOURCE exists",
			Status:  Warning,
			Message: "cannot prompt for value — no confirm function provided",
		}
	}

	value, err := confirmFn("Enter template source repo (e.g. eddiecarpenter/agentic-development)")
	if err != nil {
		return CheckResult{
			Name:    "TEMPLATE_SOURCE exists",
			Status:  Warning,
			Message: fmt.Sprintf("prompt failed: %v", err),
		}
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return CheckResult{
			Name:    "TEMPLATE_SOURCE exists",
			Status:  Warning,
			Message: "no value provided",
		}
	}

	path := filepath.Join(root, "TEMPLATE_SOURCE")
	if err := os.WriteFile(path, []byte(value+"\n"), 0o644); err != nil {
		return CheckResult{
			Name:    "TEMPLATE_SOURCE exists",
			Status:  Warning,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}
	return CheckResult{
		Name:   "TEMPLATE_SOURCE exists",
		Status: Pass,
	}
}

// RepairTEMPLATEVERSION fetches the latest tag from the template repo and writes it.
// run is injected so tests can substitute a fake implementation.
func RepairTEMPLATEVERSION(root string, run bootstrap.RunCommandFunc) CheckResult {
	// First check if TEMPLATE_SOURCE exists — we need it to fetch the version.
	sourcePath := filepath.Join(root, "TEMPLATE_SOURCE")
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: "cannot determine version — TEMPLATE_SOURCE is missing",
		}
	}

	repo := strings.TrimSpace(string(data))
	if repo == "" {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: "TEMPLATE_SOURCE is empty",
		}
	}

	// Fetch latest release tag.
	out, err := run("gh", "release", "list", "--repo", repo, "--limit", "1", "--json", "tagName", "--jq", ".[0].tagName")
	if err != nil {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: fmt.Sprintf("failed to fetch latest tag: %v", err),
		}
	}

	tag := strings.TrimSpace(out)
	if tag == "" {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: "no releases found in template repo",
		}
	}

	path := filepath.Join(root, "TEMPLATE_VERSION")
	if err := os.WriteFile(path, []byte(tag+"\n"), 0o644); err != nil {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}

	return CheckResult{
		Name:   "TEMPLATE_VERSION exists",
		Status: Pass,
	}
}

// RepairREPOSMD creates an empty REPOS.md with embedded topology comment.
func RepairREPOSMD(root string) CheckResult {
	path := filepath.Join(root, "REPOS.md")
	if err := os.WriteFile(path, []byte(emptyREPOSMD), 0o644); err != nil {
		return CheckResult{
			Name:    "REPOS.md exists",
			Status:  Fail,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}
	return CheckResult{
		Name:   "REPOS.md exists",
		Status: Pass,
	}
}

// RepairREADMEMD creates a minimal README.md with placeholder content.
func RepairREADMEMD(root string) CheckResult {
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte(minimalREADMEMD), 0o644); err != nil {
		return CheckResult{
			Name:    "README.md exists",
			Status:  Fail,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}
	return CheckResult{
		Name:   "README.md exists",
		Status: Pass,
	}
}
