package project

// GraphQL connection pagination. Lives here rather than in api.go so it stays
// covered by the project package's test suite — api.go is excluded from coverage
// as network-bound (see sonar-project.properties), and that exclusion is correct
// for client construction but not for the loop below.
//
// This helper abstracts the loop control flow only. Callers keep their own typed
// response structs and do their own accumulation: the response shapes are
// anonymous nested structs, so a generic decoder over them would cost more than
// the few lines it replaces.

// pageInfo is the subset of a GraphQL connection's pageInfo that pagination needs.
// Embed it in a response struct's connection to have it decoded.
type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

// paginate drives a cursor loop over a GraphQL connection. It calls fetchPage
// with a nil cursor first (GraphQL `after: null`), then with each successive
// endCursor, until the connection is exhausted.
//
// fetchPage performs one request, accumulates that page's nodes into whatever
// the caller is building, and returns the page's pageInfo.
//
// Any error from fetchPage aborts the loop immediately. Callers must discard
// whatever they accumulated and return nothing alongside the error: presenting a
// partially-fetched connection as a complete one is the defect this helper
// exists to prevent.
func paginate(fetchPage func(after interface{}) (pageInfo, error)) error {
	var after interface{} // nil on the first request

	for {
		pi, err := fetchPage(after)
		if err != nil {
			return err
		}

		// Stop when the connection says there is no more, and also when it
		// claims another page but hands back no cursor to fetch it with —
		// without that guard a malformed response loops forever.
		if !pi.HasNextPage || pi.EndCursor == "" {
			return nil
		}
		after = pi.EndCursor
	}
}
