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
