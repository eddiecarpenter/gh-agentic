package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// TestStatusJSON_FeaturesListDoesNotLeakInternalFields verifies the
// `gh agentic status features --json` envelope contains exactly the keys
// locked by feature #492 and no new keys — especially the internal
// TasksTotal / TasksDone counts introduced for kanban rendering.
//
// Consumers compute progress themselves from the `tasks` array on the
// detail payload; the list envelope must remain byte-compatible.
func TestStatusJSON_FeaturesListDoesNotLeakInternalFields(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{json: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeatures --json: %v", err)
	}
	raw := buf.String()
	for _, forbidden := range []string{
		"tasks_total",
		"tasks_done",
		"TasksTotal",
		"TasksDone",
	} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("features list --json must not contain %q; got:\n%s", forbidden, raw)
		}
	}

	// Structural check — the item keys are exactly the locked set.
	var payload struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v; raw:\n%s", err, raw)
	}
	lockedKeys := map[string]struct{}{
		"number":               {},
		"title":                {},
		"body":                 {},
		"stage":                {},
		"created_at":           {},
		"last_transitioned_at": {},
		"owning_repo":          {},
		"blocked":              {},
		"parent_requirement":   {},
		"tasks":                {},
		"branch":               {},
		"pr":                   {},
	}
	for i, item := range payload.Items {
		for k := range item {
			if _, ok := lockedKeys[k]; !ok {
				t.Errorf("item[%d]: unexpected key %q in JSON envelope", i, k)
			}
		}
	}
}

// TestStatusJSON_FeatureDetailDoesNotLeakInternalFields verifies the single
// feature detail `--json` payload contains exactly the locked keys.
func TestStatusJSON_FeatureDetailDoesNotLeakInternalFields(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusFeature(buf, io.Discard, 492, statusDetailFlags{json: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeature --json: %v", err)
	}
	raw := buf.String()
	for _, forbidden := range []string{
		"tasks_total",
		"tasks_done",
		"TasksTotal",
		"TasksDone",
	} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("feature detail --json must not contain %q; got:\n%s", forbidden, raw)
		}
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v; raw:\n%s", err, raw)
	}
	lockedKeys := map[string]struct{}{
		"number":               {},
		"title":                {},
		"body":                 {},
		"stage":                {},
		"created_at":           {},
		"last_transitioned_at": {},
		"owning_repo":          {},
		"blocked":              {},
		"parent_requirement":   {},
		"tasks":                {},
		"branch":               {},
		"pr":                   {},
	}
	for k := range payload {
		if _, ok := lockedKeys[k]; !ok {
			t.Errorf("feature detail: unexpected key %q in JSON payload", k)
		}
	}
}
