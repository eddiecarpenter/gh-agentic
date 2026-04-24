package scope

import (
	"testing"
)

const (
	testOwner        = "acme-org"
	testRepoFullName = "acme-org/charging-cp"
)

// allSharedNames is the canonical list of shared variable/secret names.
// Kept in a slice (not the set) so tests iterate deterministically and the
// exhaustiveness assertion can compare lengths.
var allSharedNames = []string{
	"AGENT_USER",
	"RUNNER_LABEL",
	"AGENT_PROVIDER",
	"AGENT_MODEL",
	"GOOSE_AGENT_PAT",
	"CLAUDE_CREDENTIALS_JSON",
}

// allIdentityNames is the canonical list of per-repo identity names.
var allIdentityNames = []string{
	"AGENTIC_PROJECT_ID",
	"AGENTIC_TOPOLOGY",
	"AGENTIC_FRAMEWORK_VERSION",
}

// federatedTopologies enumerates every topology string that must route
// shared values to --org.
var federatedTopologies = []string{"federated", "federated-cp", "federated-domain"}

func TestScopeFor_SharedName_FederatedTopology_RoutesToOrg(t *testing.T) {
	for _, topo := range federatedTopologies {
		for _, name := range allSharedNames {
			t.Run(topo+"/"+name, func(t *testing.T) {
				flag, target := ScopeFor(name, topo, testOwner, testRepoFullName)
				if flag != ScopeFlagOrg {
					t.Fatalf("flag: got %q, want %q", flag, ScopeFlagOrg)
				}
				if target != testOwner {
					t.Fatalf("target: got %q, want %q", target, testOwner)
				}
			})
		}
	}
}

func TestScopeFor_SharedName_SingleTopology_RoutesToRepo(t *testing.T) {
	for _, name := range allSharedNames {
		t.Run(name, func(t *testing.T) {
			flag, target := ScopeFor(name, "single", testOwner, testRepoFullName)
			if flag != ScopeFlagRepo {
				t.Fatalf("flag: got %q, want %q", flag, ScopeFlagRepo)
			}
			if target != testRepoFullName {
				t.Fatalf("target: got %q, want %q", target, testRepoFullName)
			}
		})
	}
}

func TestScopeFor_IdentityName_AlwaysRoutesToRepo(t *testing.T) {
	topos := append([]string{"single", "", "mystery"}, federatedTopologies...)
	for _, topo := range topos {
		for _, name := range allIdentityNames {
			t.Run(topo+"/"+name, func(t *testing.T) {
				flag, target := ScopeFor(name, topo, testOwner, testRepoFullName)
				if flag != ScopeFlagRepo {
					t.Fatalf("flag: got %q, want %q", flag, ScopeFlagRepo)
				}
				if target != testRepoFullName {
					t.Fatalf("target: got %q, want %q", target, testRepoFullName)
				}
			})
		}
	}
}

func TestScopeFor_UnknownName_RoutesToRepo(t *testing.T) {
	topos := append([]string{"single", "", "mystery"}, federatedTopologies...)
	for _, topo := range topos {
		t.Run(topo, func(t *testing.T) {
			flag, target := ScopeFor("SOMETHING_ELSE", topo, testOwner, testRepoFullName)
			if flag != ScopeFlagRepo {
				t.Fatalf("flag: got %q, want %q", flag, ScopeFlagRepo)
			}
			if target != testRepoFullName {
				t.Fatalf("target: got %q, want %q", target, testRepoFullName)
			}
		})
	}
}

func TestScopeFor_UnknownTopology_DefaultsToRepo(t *testing.T) {
	// All nine declared names must stay at --repo under an unknown topology.
	allDeclared := append([]string{}, allSharedNames...)
	allDeclared = append(allDeclared, allIdentityNames...)

	for _, topo := range []string{"", "mystery", "SINGLE", "Federated"} { // case-sensitive: upper-case is unknown
		for _, name := range allDeclared {
			t.Run(topo+"/"+name, func(t *testing.T) {
				flag, target := ScopeFor(name, topo, testOwner, testRepoFullName)
				if flag != ScopeFlagRepo {
					t.Fatalf("flag: got %q, want %q", flag, ScopeFlagRepo)
				}
				if target != testRepoFullName {
					t.Fatalf("target: got %q, want %q", target, testRepoFullName)
				}
			})
		}
	}
}

func TestIsSharedName_KnownNames_ReturnsTrue(t *testing.T) {
	for _, name := range allSharedNames {
		t.Run(name, func(t *testing.T) {
			if !IsSharedName(name) {
				t.Fatalf("IsSharedName(%q) = false, want true", name)
			}
		})
	}
}

func TestIsSharedName_UnknownNames_ReturnsFalse(t *testing.T) {
	cases := []string{"", "AGENTIC_PROJECT_ID", "AGENTIC_TOPOLOGY", "AGENTIC_FRAMEWORK_VERSION", "RANDOM_NAME"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			if IsSharedName(name) {
				t.Fatalf("IsSharedName(%q) = true, want false", name)
			}
		})
	}
}

func TestIsIdentityName_KnownNames_ReturnsTrue(t *testing.T) {
	for _, name := range allIdentityNames {
		t.Run(name, func(t *testing.T) {
			if !IsIdentityName(name) {
				t.Fatalf("IsIdentityName(%q) = false, want true", name)
			}
		})
	}
}

func TestIsIdentityName_UnknownNames_ReturnsFalse(t *testing.T) {
	cases := append([]string{"", "RANDOM_NAME"}, allSharedNames...)
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			if IsIdentityName(name) {
				t.Fatalf("IsIdentityName(%q) = true, want false", name)
			}
		})
	}
}

func TestSharedAndIdentity_AreMutuallyExclusive(t *testing.T) {
	for _, name := range allSharedNames {
		if IsIdentityName(name) {
			t.Fatalf("%q is both shared and identity", name)
		}
	}
	for _, name := range allIdentityNames {
		if IsSharedName(name) {
			t.Fatalf("%q is both identity and shared", name)
		}
	}
}

func TestSharedAndIdentity_AreExhaustive(t *testing.T) {
	// Guard against accidentally adding a name to one map but forgetting the
	// counterpart test list — the maps and the test slices must agree.
	if got, want := len(sharedNames), len(allSharedNames); got != want {
		t.Fatalf("sharedNames count: got %d, want %d (test list out of sync)", got, want)
	}
	if got, want := len(identityNames), len(allIdentityNames); got != want {
		t.Fatalf("identityNames count: got %d, want %d (test list out of sync)", got, want)
	}
}
