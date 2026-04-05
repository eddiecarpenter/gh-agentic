package bootstrap

import (
	"errors"
	"testing"
)

func TestDetectOwnerType_FakeImplementation(t *testing.T) {
	tests := []struct {
		name       string
		fake       DetectOwnerTypeFunc
		owner      string
		wantType   string
		wantErr    bool
	}{
		{
			name: "personal account returns OwnerTypeUser",
			fake: func(owner string) (string, error) {
				return OwnerTypeUser, nil
			},
			owner:    "alice",
			wantType: OwnerTypeUser,
			wantErr:  false,
		},
		{
			name: "organisation returns OwnerTypeOrg",
			fake: func(owner string) (string, error) {
				return OwnerTypeOrg, nil
			},
			owner:    "acme-corp",
			wantType: OwnerTypeOrg,
			wantErr:  false,
		},
		{
			name: "API error returns error",
			fake: func(owner string) (string, error) {
				return "", errors.New("API rate limit exceeded")
			},
			owner:   "unknown",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.fake(tc.owner)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantType {
				t.Errorf("got %q, want %q", got, tc.wantType)
			}
		})
	}
}

func TestOwnerTypeConstants(t *testing.T) {
	if OwnerTypeUser != "User" {
		t.Errorf("OwnerTypeUser = %q, want %q", OwnerTypeUser, "User")
	}
	if OwnerTypeOrg != "Organization" {
		t.Errorf("OwnerTypeOrg = %q, want %q", OwnerTypeOrg, "Organization")
	}
}
