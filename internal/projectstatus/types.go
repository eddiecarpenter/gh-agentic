// Package projectstatus provides the data model and GitHub queries that back
// the `gh agentic status` command group. The CLI layer consumes the typed
// values defined here — renderers read them, they do not talk to GitHub.
//
// All query functions accept injectable dependency types (see deps.go) so
// tests can substitute fakes; the package is exercised entirely without
// network access.
package projectstatus

import (
	"strings"
	"time"
)

// Stage is the canonical, lower-kebab-case form of a pipeline stage.
// It matches the `Status` field option names on the GitHub ProjectV2 board
// (normalised to lower kebab case) and the label-derived stage names.
type Stage string

// Canonical pipeline stages. StageUnknown is the zero value returned by
// ParseStage when it cannot recognise the input — callers treat it as
// "stage missing" rather than falling back to a guess.
const (
	StageBacklog       Stage = "backlog"
	StageScoping       Stage = "scoping"
	StageScheduled     Stage = "scheduled"
	StageInDesign      Stage = "in-design"
	StageInDevelopment Stage = "in-development"
	StageInReview      Stage = "in-review"
	StageDone          Stage = "done"
	StageUnknown       Stage = ""
)

// String returns the stage in canonical lower-kebab form.
func (s Stage) String() string {
	return string(s)
}

// ParseStage accepts a GitHub ProjectV2 Status option name, a label name, or
// already-canonical form, and returns the canonical Stage. The comparison is
// case-insensitive and tolerates a space between words (e.g. "In Design")
// or the hyphenated form ("in-design"). Unknown input returns StageUnknown.
//
// This is the single chokepoint between external label/option names and the
// pipeline-stage enum — every caller in the projectstatus package should use
// it rather than string matching ad hoc.
func ParseStage(raw string) Stage {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, " ", "-")
	switch s {
	case "backlog":
		return StageBacklog
	case "scoping":
		return StageScoping
	case "scheduled":
		return StageScheduled
	case "in-design":
		return StageInDesign
	case "in-development":
		return StageInDevelopment
	case "in-review":
		return StageInReview
	case "done":
		return StageDone
	default:
		return StageUnknown
	}
}

// Requirement is a single requirement issue with its pipeline state and
// linked features. Fields match the JSON schema locked in acceptance criteria.
type Requirement struct {
	Number             int
	Title              string
	Body               string
	Stage              Stage
	CreatedAt          time.Time
	LastTransitionedAt time.Time
	OwningRepo         string // "owner/name"
	Blocked            *BlockedInfo
	LinkedFeatures     []FeatureSummary
}

// Feature is a single feature issue with its pipeline state, parent
// requirement reference, tasks, branch state, and PR state.
type Feature struct {
	Number             int
	Title              string
	Body               string
	Stage              Stage
	CreatedAt          time.Time
	LastTransitionedAt time.Time
	OwningRepo         string
	Blocked            *BlockedInfo
	ParentRequirement  *RequirementSummary
	Tasks              []TaskRef
	Branch             *BranchState
	PR                 *PRState
}

// RequirementSummary is the compact embedded form used when a feature
// references its parent requirement.
type RequirementSummary struct {
	Number     int
	Title      string
	Stage      Stage
	OwningRepo string
}

// FeatureSummary is the compact embedded form used when a requirement
// references its linked features. BranchOneLiner is a pre-rendered string
// (e.g. "feature/x (merged)") for human list views; PR is the structured
// state used by both list and JSON views.
type FeatureSummary struct {
	Number         int
	Title          string
	Stage          Stage
	OwningRepo     string
	BranchOneLiner string
	PR             *PRState
}

// TaskRef is a sub-issue reference used when showing the task checklist of a
// feature. Closed drives the ✓/☐ glyph in human output.
type TaskRef struct {
	Number int
	Title  string
	Closed bool
}

// BranchState describes whether a feature branch exists and whether it has
// been merged. Exists=false means no ref matches the expected branch name;
// Merged=true means the ref (or a PR from it) has been merged into the default
// branch.
type BranchState struct {
	Name   string
	Exists bool
	Merged bool
}

// PRState describes the PR associated with a feature branch. State is one of
// "open", "merged", "closed".
type PRState struct {
	Number    int
	State     string
	Reviewers []string
}

// BlockedInfo describes the dependency that blocks an issue. BlockingRef is a
// stable string in "owner/repo#N" form; Reason is optional free-form text
// supplied by the dependency mechanism.
type BlockedInfo struct {
	BlockingRef string
	Reason      string
}
