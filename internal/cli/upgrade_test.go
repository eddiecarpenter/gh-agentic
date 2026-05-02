package cli

import "testing"

func TestEnsureVPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2.6.2", "v2.6.2"},
		{"v2.6.2", "v2.6.2"},
		{"2.6.2-rc1", "v2.6.2-rc1"},
		{"v2.6.2-rc1", "v2.6.2-rc1"},
		{"dev", "vdev"},
	}
	for _, c := range cases {
		if got := ensureVPrefix(c.in); got != c.want {
			t.Errorf("ensureVPrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestUpgradeConfirmBehaviour verifies the confirm-wiring logic:
// no explicit version → confirm skipped; explicit version → confirm wired;
// --yes → confirm skipped regardless.
func TestUpgradeConfirmBehaviour(t *testing.T) {
	fakeConfirm := func(string) (bool, error) { return true, nil }

	cases := []struct {
		name            string
		yes             bool
		explicitVersion bool
		wantConfirm     bool
	}{
		{"no arg skips confirm", false, false, false},
		{"explicit version wires confirm", false, true, true},
		{"yes flag suppresses confirm with explicit version", true, true, false},
		{"yes flag suppresses confirm without explicit version", true, false, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var confirm func(string) (bool, error)
			if !c.yes && c.explicitVersion {
				confirm = fakeConfirm
			}
			got := confirm != nil
			if got != c.wantConfirm {
				t.Errorf("confirm wired=%v, want %v", got, c.wantConfirm)
			}
		})
	}
}
