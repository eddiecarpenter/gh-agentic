package bootstrap

// OwnerType constants identify whether a GitHub owner is a personal account or an organisation.
const (
	OwnerTypeUser = "User"
	OwnerTypeOrg  = "Organization"
)

// DetectOwnerTypeFunc detects whether a GitHub owner is a personal account or an organisation.
// Injected so tests can substitute a fake implementation without real gh auth.
type DetectOwnerTypeFunc func(owner string) (string, error)
