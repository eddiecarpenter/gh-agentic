package sync

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestRelease_JSONUnmarshal_PopulatesTarballURL(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantURL    string
		wantTag    string
		wantName   string
		wantBody   string
	}{
		{
			name:     "full GitHub release response populates TarballURL",
			input:    `{"tag_name":"v1.0.0","name":"Release 1.0.0","body":"Release notes here","tarball_url":"https://api.github.com/repos/owner/repo/tarball/v1.0.0"}`,
			wantURL:  "https://api.github.com/repos/owner/repo/tarball/v1.0.0",
			wantTag:  "v1.0.0",
			wantName: "Release 1.0.0",
			wantBody: "Release notes here",
		},
		{
			name:     "missing tarball_url leaves field empty",
			input:    `{"tag_name":"v0.9.0","name":"Old release","body":"Notes"}`,
			wantURL:  "",
			wantTag:  "v0.9.0",
			wantName: "Old release",
			wantBody: "Notes",
		},
		{
			name:     "empty tarball_url",
			input:    `{"tag_name":"v2.0.0","name":"Release 2","body":"","tarball_url":""}`,
			wantURL:  "",
			wantTag:  "v2.0.0",
			wantName: "Release 2",
			wantBody: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var r Release
			if err := json.Unmarshal([]byte(tc.input), &r); err != nil {
				t.Fatalf("unexpected unmarshal error: %v", err)
			}

			if r.TarballURL != tc.wantURL {
				t.Errorf("TarballURL = %q, want %q", r.TarballURL, tc.wantURL)
			}
			if r.TagName != tc.wantTag {
				t.Errorf("TagName = %q, want %q", r.TagName, tc.wantTag)
			}
			if r.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", r.Name, tc.wantName)
			}
			if r.Body != tc.wantBody {
				t.Errorf("Body = %q, want %q", r.Body, tc.wantBody)
			}
		})
	}
}

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
			repo: "eddiecarpenter/ai-native-delivery",
			fetchFunc: func(repo string) (string, error) {
				if repo != "eddiecarpenter/ai-native-delivery" {
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

func TestFilterReleasesSince(t *testing.T) {
	releases := []Release{
		{TagName: "v0.9.5", Name: "Older release", Body: "Notes for v0.9.5"},
		{TagName: "v0.9.8", Name: "Newest release", Body: "Notes for v0.9.8"},
		{TagName: "v0.9.6", Name: "Middle release", Body: "Notes for v0.9.6"},
		{TagName: "v0.9.7", Name: "Recent release", Body: "Notes for v0.9.7"},
	}

	tests := []struct {
		name     string
		since    string
		wantTags []string
	}{
		{
			name:     "filters releases newer than v0.9.5",
			since:    "v0.9.5",
			wantTags: []string{"v0.9.8", "v0.9.7", "v0.9.6"},
		},
		{
			name:     "filters releases newer than v0.9.7",
			since:    "v0.9.7",
			wantTags: []string{"v0.9.8"},
		},
		{
			name:     "no newer releases returns empty",
			since:    "v0.9.8",
			wantTags: nil,
		},
		{
			name:     "handles version without v prefix",
			since:    "0.9.6",
			wantTags: []string{"v0.9.8", "v0.9.7"},
		},
		{
			name:     "returns newest-first order",
			since:    "v0.9.4",
			wantTags: []string{"v0.9.8", "v0.9.7", "v0.9.6", "v0.9.5"},
		},
		{
			name:     "invalid since version returns nil",
			since:    "not-a-version",
			wantTags: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterReleasesSince(releases, tc.since)

			if len(got) != len(tc.wantTags) {
				t.Fatalf("got %d releases, want %d", len(got), len(tc.wantTags))
			}

			for i, r := range got {
				if r.TagName != tc.wantTags[i] {
					t.Errorf("release[%d].TagName = %q, want %q", i, r.TagName, tc.wantTags[i])
				}
			}
		})
	}
}

func TestFilterReleasesSince_EmptySlice(t *testing.T) {
	got := FilterReleasesSince(nil, "v1.0.0")
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d releases", len(got))
	}
}

func TestFindReleaseByTag(t *testing.T) {
	releases := []Release{
		{TagName: "v0.9.5", Name: "Release 5", Body: "Notes 5"},
		{TagName: "v0.9.6", Name: "Release 6", Body: "Notes 6"},
		{TagName: "v0.9.7", Name: "Release 7", Body: "Notes 7"},
	}

	tests := []struct {
		name      string
		tag       string
		wantFound bool
		wantName  string
	}{
		{
			name:      "finds existing tag",
			tag:       "v0.9.6",
			wantFound: true,
			wantName:  "Release 6",
		},
		{
			name:      "finds first tag",
			tag:       "v0.9.5",
			wantFound: true,
			wantName:  "Release 5",
		},
		{
			name:      "finds last tag",
			tag:       "v0.9.7",
			wantFound: true,
			wantName:  "Release 7",
		},
		{
			name:      "tag not found",
			tag:       "v1.0.0",
			wantFound: false,
		},
		{
			name:      "empty tag not found",
			tag:       "",
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := FindReleaseByTag(releases, tc.tag)

			if found != tc.wantFound {
				t.Fatalf("found = %v, want %v", found, tc.wantFound)
			}

			if tc.wantFound && got.Name != tc.wantName {
				t.Errorf("got.Name = %q, want %q", got.Name, tc.wantName)
			}
		})
	}
}

func TestFindReleaseByTag_EmptySlice(t *testing.T) {
	_, found := FindReleaseByTag(nil, "v1.0.0")
	if found {
		t.Error("expected not found for empty slice")
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
