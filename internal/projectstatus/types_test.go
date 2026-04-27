package projectstatus

import (
	"testing"
)

// TestParseStage_AllKnownForms verifies every recognised input maps to the
// canonical Stage constant, covering both ProjectV2 Status option names
// (space-separated) and already-canonical label names.
func TestParseStage_AllKnownForms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Stage
	}{
		{name: "backlog lower", input: "backlog", expected: StageBacklog},
		{name: "Backlog title", input: "Backlog", expected: StageBacklog},
		{name: "  Backlog  whitespace", input: "  Backlog  ", expected: StageBacklog},
		{name: "scoping", input: "Scoping", expected: StageScoping},
		{name: "ready-to-implement hyphenated", input: "ready-to-implement", expected: StageReadyToImplement},
		{name: "Ready to Implement spaced", input: "Ready to Implement", expected: StageReadyToImplement},
		{name: "in design with space", input: "In Design", expected: StageInDesign},
		{name: "in-design hyphenated", input: "in-design", expected: StageInDesign},
		{name: "IN DESIGN upper", input: "IN DESIGN", expected: StageInDesign},
		{name: "designed lower", input: "designed", expected: StageDesigned},
		{name: "Designed title", input: "Designed", expected: StageDesigned},
		{name: "in development space", input: "In Development", expected: StageInDevelopment},
		{name: "in-development hyphenated", input: "in-development", expected: StageInDevelopment},
		{name: "in review space", input: "In Review", expected: StageInReview},
		{name: "in-review hyphenated", input: "in-review", expected: StageInReview},
		{name: "done", input: "Done", expected: StageDone},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseStage(tc.input)
			if got != tc.expected {
				t.Errorf("ParseStage(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

// TestParseStage_UnknownInputs verifies unrecognised input degrades to
// StageUnknown rather than guessing.
func TestParseStage_UnknownInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "whitespace", input: "   "},
		{name: "partial", input: "in"},
		{name: "junk", input: "not-a-stage"},
		{name: "similar but wrong", input: "reviewing"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseStage(tc.input)
			if got != StageUnknown {
				t.Errorf("ParseStage(%q) = %q, want StageUnknown", tc.input, got)
			}
		})
	}
}

// TestStage_String_ReturnsCanonicalForm verifies the String method round-trips.
func TestStage_String_ReturnsCanonicalForm(t *testing.T) {
	cases := []Stage{StageBacklog, StageScoping, StageReadyToImplement, StageInDesign,
		StageDesigned, StageInDevelopment, StageInReview, StageDone}
	for _, s := range cases {
		got := s.String()
		if got != string(s) {
			t.Errorf("%q.String() = %q, want %q", s, got, string(s))
		}
	}
}
