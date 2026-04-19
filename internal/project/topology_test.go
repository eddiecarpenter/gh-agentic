package project

import (
	"errors"
	"testing"
)

// variableStore is a tiny helper that backs a fake GetRepoVariableFunc
// with an in-memory map so tests can assert which variables were read.
type variableStore struct {
	values map[string]string
	reads  map[string]int
	readsN int
}

func newVariableStore(values map[string]string) *variableStore {
	return &variableStore{
		values: values,
		reads:  map[string]int{},
	}
}

func (s *variableStore) get(owner, repo, name string) (string, error) {
	s.readsN++
	s.reads[name]++
	if v, ok := s.values[name]; ok {
		return v, nil
	}
	return "", errors.New("variable not found")
}

// countingFetchLinkedRepos wraps a fetch result and counts invocations so
// tests can assert the resolver never calls the network twice.
func countingFetchLinkedRepos(repos []LinkedRepo, err error, calls *int) FetchLinkedReposFunc {
	return func(projectID string) ([]LinkedRepo, error) {
		*calls++
		return repos, err
	}
}

func TestResolveTopology_FederatedVariable_WithFrameworkVersion_ReturnsFederatedCP(t *testing.T) {
	store := newVariableStore(map[string]string{
		TopologyVarName:         "federated",
		FrameworkVersionVarName: "v2.1.0",
	})

	got, err := ResolveTopology(ResolveTopologyDeps{
		Owner:           "org",
		Repo:            "control-plane",
		ProjectID:       "PVT_cp",
		GetRepoVariable: store.get,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != TopologyStringFederatedCP {
		t.Errorf("got %q, want %q", got, TopologyStringFederatedCP)
	}
}

func TestResolveTopology_FederatedVariable_NoFrameworkVersion_ReturnsFederatedDomain(t *testing.T) {
	store := newVariableStore(map[string]string{
		TopologyVarName: "federated",
	})

	got, err := ResolveTopology(ResolveTopologyDeps{
		Owner:           "org",
		Repo:            "domain-repo",
		ProjectID:       "PVT_dom",
		GetRepoVariable: store.get,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != TopologyStringFederatedDomain {
		t.Errorf("got %q, want %q", got, TopologyStringFederatedDomain)
	}
}

func TestResolveTopology_SingleVariable_ReturnsSingle(t *testing.T) {
	store := newVariableStore(map[string]string{
		TopologyVarName: "single",
	})

	got, err := ResolveTopology(ResolveTopologyDeps{
		Owner:           "user",
		Repo:            "solo",
		ProjectID:       "PVT_solo",
		GetRepoVariable: store.get,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != TopologyStringSingle {
		t.Errorf("got %q, want %q", got, TopologyStringSingle)
	}
}

func TestResolveTopology_VariableUnset_ManyLinkedRepos_NoLocalVersion_ReturnsFederatedDomain(t *testing.T) {
	// Exactly the charging-domain scenario: no local AGENTIC_TOPOLOGY,
	// no local AGENTIC_FRAMEWORK_VERSION, but the project has multiple
	// linked repos (CP plus domain(s)).
	store := newVariableStore(map[string]string{})
	fetchCalls := 0
	fetch := countingFetchLinkedRepos(
		[]LinkedRepo{
			{NameWithOwner: "org/control-plane"},
			{NameWithOwner: "org/domain-one"},
			{NameWithOwner: "org/domain-two"},
		}, nil, &fetchCalls,
	)

	got, err := ResolveTopology(ResolveTopologyDeps{
		Owner:            "org",
		Repo:             "domain-one",
		ProjectID:        "PVT_charging",
		GetRepoVariable:  store.get,
		FetchLinkedRepos: fetch,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != TopologyStringFederatedDomain {
		t.Errorf("got %q, want %q", got, TopologyStringFederatedDomain)
	}
	if fetchCalls != 1 {
		t.Errorf("FetchLinkedRepos calls: got %d, want 1 (must cache)", fetchCalls)
	}
}

func TestResolveTopology_VariableUnset_ManyLinkedRepos_WithLocalVersion_ReturnsFederatedCP(t *testing.T) {
	store := newVariableStore(map[string]string{
		FrameworkVersionVarName: "v2.1.0",
	})
	fetchCalls := 0
	fetch := countingFetchLinkedRepos(
		[]LinkedRepo{
			{NameWithOwner: "org/control-plane"},
			{NameWithOwner: "org/domain-one"},
		}, nil, &fetchCalls,
	)

	got, err := ResolveTopology(ResolveTopologyDeps{
		Owner:            "org",
		Repo:             "control-plane",
		ProjectID:        "PVT_fed",
		GetRepoVariable:  store.get,
		FetchLinkedRepos: fetch,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != TopologyStringFederatedCP {
		t.Errorf("got %q, want %q", got, TopologyStringFederatedCP)
	}
	if fetchCalls != 1 {
		t.Errorf("FetchLinkedRepos calls: got %d, want 1", fetchCalls)
	}
}

func TestResolveTopology_VariableUnset_ZeroOrOneLinked_NoVersion_ReturnsSingle(t *testing.T) {
	tests := []struct {
		name  string
		repos []LinkedRepo
	}{
		{name: "zero linked", repos: nil},
		{name: "one linked", repos: []LinkedRepo{{NameWithOwner: "user/solo"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newVariableStore(map[string]string{})
			fetchCalls := 0
			fetch := countingFetchLinkedRepos(tc.repos, nil, &fetchCalls)

			got, err := ResolveTopology(ResolveTopologyDeps{
				Owner:            "user",
				Repo:             "solo",
				ProjectID:        "PVT_solo",
				GetRepoVariable:  store.get,
				FetchLinkedRepos: fetch,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != TopologyStringSingle {
				t.Errorf("got %q, want %q", got, TopologyStringSingle)
			}
			if fetchCalls != 1 {
				t.Errorf("FetchLinkedRepos calls: got %d, want 1", fetchCalls)
			}
		})
	}
}

func TestResolveTopology_NoProjectID_ReturnsSingle(t *testing.T) {
	// An un-affiliated repo: no AGENTIC_PROJECT_ID set, nothing to inspect.
	// The resolver must not attempt to call FetchLinkedRepos.
	store := newVariableStore(map[string]string{})
	fetchCalls := 0
	fetch := countingFetchLinkedRepos(nil, errors.New("should not be called"), &fetchCalls)

	got, err := ResolveTopology(ResolveTopologyDeps{
		Owner:            "user",
		Repo:             "stray",
		ProjectID:        "",
		GetRepoVariable:  store.get,
		FetchLinkedRepos: fetch,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != TopologyStringSingle {
		t.Errorf("got %q, want %q", got, TopologyStringSingle)
	}
	if fetchCalls != 0 {
		t.Errorf("FetchLinkedRepos must not be called when ProjectID is empty; got %d calls", fetchCalls)
	}
}

func TestResolveTopology_FetchLinkedReposError_PropagatesError(t *testing.T) {
	store := newVariableStore(map[string]string{})
	wantErr := errors.New("graphql boom")
	fetchCalls := 0
	fetch := countingFetchLinkedRepos(nil, wantErr, &fetchCalls)

	got, err := ResolveTopology(ResolveTopologyDeps{
		Owner:            "org",
		Repo:             "domain",
		ProjectID:        "PVT_broken",
		GetRepoVariable:  store.get,
		FetchLinkedRepos: fetch,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error: got %v, want %v", err, wantErr)
	}
	if got != "" {
		t.Errorf("on error, topology should be empty; got %q", got)
	}
	if fetchCalls != 1 {
		t.Errorf("FetchLinkedRepos calls: got %d, want 1", fetchCalls)
	}
}

func TestResolveTopology_NilGetRepoVariable_FallsBackToSingle(t *testing.T) {
	// Defensive path: without any way to read variables the resolver
	// cannot inspect local signals, so the safe default is single.
	got, err := ResolveTopology(ResolveTopologyDeps{
		Owner:     "user",
		Repo:      "solo",
		ProjectID: "PVT_solo",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != TopologyStringSingle {
		t.Errorf("got %q, want %q", got, TopologyStringSingle)
	}
}

func TestResolveTopology_NilFetchLinkedRepos_FallsBackToSingle(t *testing.T) {
	// If the caller forgets to wire FetchLinkedRepos but the variable
	// is unset, we cannot inspect the project — fall back safely.
	store := newVariableStore(map[string]string{})

	got, err := ResolveTopology(ResolveTopologyDeps{
		Owner:           "user",
		Repo:            "solo",
		ProjectID:       "PVT_solo",
		GetRepoVariable: store.get,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != TopologyStringSingle {
		t.Errorf("got %q, want %q", got, TopologyStringSingle)
	}
}

func TestResolveTopology_FederatedVariable_NoFetchNeeded(t *testing.T) {
	// When AGENTIC_TOPOLOGY is set, the linked-repos path must not be
	// exercised at all — the variable is authoritative.
	store := newVariableStore(map[string]string{
		TopologyVarName:         "federated",
		FrameworkVersionVarName: "v2.1.0",
	})
	fetchCalls := 0
	fetch := countingFetchLinkedRepos(nil, errors.New("should not be called"), &fetchCalls)

	_, err := ResolveTopology(ResolveTopologyDeps{
		Owner:            "org",
		Repo:             "control-plane",
		ProjectID:        "PVT_cp",
		GetRepoVariable:  store.get,
		FetchLinkedRepos: fetch,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetchCalls != 0 {
		t.Errorf("FetchLinkedRepos must not be called on the variable-set path; got %d", fetchCalls)
	}
}
