package project

// Canonical topology strings used throughout gh-agentic.
//
// Topology is derived from FEDERATION.md presence at the repo root:
//   - FEDERATION.md present → TopologyStringFederation ("federation")
//   - FEDERATION.md absent  → TopologyStringSingle ("single")
//
// The three-way split (single / federated-cp / federated-domain) that was
// previously inferred from AGENTIC_TOPOLOGY variables and linked-repo graph
// inspection has been replaced by binary manifest presence. See Feature #824
// for the migration rationale.
const (
	TopologyStringSingle     = "single"
	TopologyStringFederation = "federation"
)
