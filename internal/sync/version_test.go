package sync

import (
	"fmt"
	"testing"
)

func TestFetchLatestRelease(t *testing.T) {
	tests := []struct {
		name      string
		repo      string
		fetchFunc FetchReleaseFunc
		want      string
		wantError bool
	}{
		{
			name: "successful fetch",
			repo: "owner/repo",
			fetchFunc: func(_ string) (string, error) {
				return "v1.2.3", nil
			},
			want: "v1.2.3",
		},
		{
			name: "fetch error",
			repo: "owner/repo",
			fetchFunc: func(_ string) (string, error) {
				return "", fmt.Errorf("API error: not found")
			},
			wantError: true,
		},
		{
			name: "empty repo",
			repo: "",
			fetchFunc: func(_ string) (string, error) {
				return "v1.0.0", nil
			},
			wantError: true,
		},
		{
			name: "passes repo to fetch function",
			repo: "eddiecarpenter/agentic-development",
			fetchFunc: func(repo string) (string, error) {
				if repo != "eddiecarpenter/agentic-development" {
					return "", fmt.Errorf("unexpected repo: %s", repo)
				}
				return "v0.3.0", nil
			},
			want: "v0.3.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FetchLatestRelease(tc.repo, tc.fetchFunc)

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

func TestIsUpToDate(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{
			name:    "matching versions",
			current: "v0.1.0",
			latest:  "v0.1.0",
			want:    true,
		},
		{
			name:    "different versions",
			current: "v0.1.0",
			latest:  "v0.2.0",
			want:    false,
		},
		{
			name:    "empty current",
			current: "",
			latest:  "v0.1.0",
			want:    false,
		},
		{
			name:    "empty latest",
			current: "v0.1.0",
			latest:  "",
			want:    false,
		},
		{
			name:    "both empty",
			current: "",
			latest:  "",
			want:    true,
		},
		{
			name:    "whitespace trimming",
			current: " v0.1.0 ",
			latest:  "v0.1.0",
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsUpToDate(tc.current, tc.latest)

			if got != tc.want {
				t.Errorf("IsUpToDate(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}
