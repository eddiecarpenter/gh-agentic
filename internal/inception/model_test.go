package inception

import "testing"

func TestFullRepoName_Domain_AppendsSuffix(t *testing.T) {
	cfg := InceptionConfig{RepoType: "domain", RepoName: "charging"}
	if got := FullRepoName(cfg); got != "charging-domain" {
		t.Errorf("FullRepoName() = %q, want %q", got, "charging-domain")
	}
}

func TestFullRepoName_Tool_AppendsSuffix(t *testing.T) {
	cfg := InceptionConfig{RepoType: "tool", RepoName: "testbench"}
	if got := FullRepoName(cfg); got != "testbench-tool" {
		t.Errorf("FullRepoName() = %q, want %q", got, "testbench-tool")
	}
}

func TestFullRepoName_Other_NoSuffix(t *testing.T) {
	cfg := InceptionConfig{RepoType: "other", RepoName: "my-service"}
	if got := FullRepoName(cfg); got != "my-service" {
		t.Errorf("FullRepoName() = %q, want %q", got, "my-service")
	}
}

func TestFullRepoName_EmptyType_NoSuffix(t *testing.T) {
	cfg := InceptionConfig{RepoType: "", RepoName: "my-service"}
	if got := FullRepoName(cfg); got != "my-service" {
		t.Errorf("FullRepoName() = %q, want %q", got, "my-service")
	}
}
