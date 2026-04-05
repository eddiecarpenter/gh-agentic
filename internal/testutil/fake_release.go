package testutil

// FakeRelease returns a closure that matches the sync.FetchReleaseFunc
// signature: func(repo string) (string, error). The closure ignores the repo
// argument and returns the given tag and err. This avoids circular imports by
// not referencing the sync package directly.
func FakeRelease(tag string, err error) func(repo string) (string, error) {
	return func(_ string) (string, error) {
		return tag, err
	}
}
