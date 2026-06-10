package project

import "errors"

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
