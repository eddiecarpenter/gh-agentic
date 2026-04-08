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

// FakeReleaseData holds test data for a single release entry. It mirrors
// sync.Release but avoids circular imports by not referencing the sync package.
// Test code converts these into sync.Release values directly.
type FakeReleaseData struct {
	TagName string
	Name    string
	Body    string
}

// SampleReleases returns a set of fake release data entries useful for testing
// multi-release scenarios. The entries span v0.9.5 to v0.9.8, ordered
// newest-first, each with a distinct name and body.
func SampleReleases() []FakeReleaseData {
	return []FakeReleaseData{
		{TagName: "v0.9.8", Name: "Fix sync runner edge case", Body: "Fixed a bug in the sync runner."},
		{TagName: "v0.9.7", Name: "Add execution guards to skills", Body: "Added guards to prevent skill re-entry."},
		{TagName: "v0.9.6", Name: "Release notes workflow and restructure", Body: "Restructured release notes generation."},
		{TagName: "v0.9.5", Name: "Initial release", Body: "First stable release."},
	}
}
