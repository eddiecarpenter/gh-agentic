package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// kanbanSchema is the parsed form of
// testdata/status_schemas/kanban_combined_envelope.schema.json. Tests
// assert key presence against this shape rather than byte-comparing
// payloads — the fixture is the authoritative key list.
type kanbanSchema struct {
	EnvelopeDefault                      map[string]string `json:"envelope_default"`
	EnvelopeRequirementsSelector         map[string]string `json:"envelope_requirements_selector"`
	EnvelopeFeaturesSelector             map[string]string `json:"envelope_features_selector"`
	TotalsFieldsDefault                  map[string]string `json:"totals_fields_default"`
	TotalsFieldsRequirementsSelector     map[string]string `json:"totals_fields_requirements_selector"`
	TotalsFieldsFeaturesSelector         map[string]string `json:"totals_fields_features_selector"`
	InnerRequirementFields               map[string]string `json:"inner_requirement_fields"`
	InnerFeatureFields                   map[string]string `json:"inner_feature_fields"`
}

// loadKanbanSchema reads the fixture once and returns a parsed struct.
// Test authors can call this from any test rather than re-parsing.
func loadKanbanSchema(t *testing.T) kanbanSchema {
	t.Helper()
	path := filepath.Join("testdata", "status_schemas", "kanban_combined_envelope.schema.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	var schema kanbanSchema
	if err := json.Unmarshal(body, &schema); err != nil {
		t.Fatalf("decode schema fixture: %v", err)
	}
	return schema
}

// keysExactly asserts that got contains exactly the keys listed in want
// — no missing keys, no unexpected extras. Fatally fails the test on
// either violation with a diagnostic that lists both sets.
func keysExactly(t *testing.T, context string, got map[string]json.RawMessage, want map[string]string) {
	t.Helper()
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("%s: missing expected key %q; got %v", context, k, keysOfRaw(got))
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s: unexpected key %q (not in schema); got %v", context, k, keysOfRaw(got))
		}
	}
}

// TestKanbanJSON_CombinedEnvelopeSchema verifies the default
// (`kanban --json`) envelope has exactly the {requirements, features,
// totals} top-level keys and the totals object has exactly
// {open_requirements, open_features, blocked}. AC-6 / AC-14 lock.
func TestKanbanJSON_CombinedEnvelopeSchema(t *testing.T) {
	schema := loadKanbanSchema(t)

	buf := &bytes.Buffer{}
	if err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{json: true}, kanbanSampleDeps()); err != nil {
		t.Fatalf("runKanban --json: %v", err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v; raw:\n%s", err, buf.String())
	}
	keysExactly(t, "default envelope", envelope, schema.EnvelopeDefault)

	var totals map[string]json.RawMessage
	if err := json.Unmarshal(envelope["totals"], &totals); err != nil {
		t.Fatalf("decode totals: %v", err)
	}
	keysExactly(t, "default totals", totals, schema.TotalsFieldsDefault)
}

// TestKanbanJSON_CombinedEnvelopeInnerRequirementFields verifies the
// per-requirement object inside the envelope uses the locked key set —
// guards against new fields leaking into the contract.
func TestKanbanJSON_CombinedEnvelopeInnerRequirementFields(t *testing.T) {
	schema := loadKanbanSchema(t)

	buf := &bytes.Buffer{}
	if err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{json: true}, kanbanSampleDeps()); err != nil {
		t.Fatalf("runKanban --json: %v", err)
	}
	var envelope struct {
		Requirements []map[string]json.RawMessage `json:"requirements"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v; raw:\n%s", err, buf.String())
	}
	if len(envelope.Requirements) == 0 {
		t.Fatalf("expected at least one requirement in sample; got 0")
	}
	for i, item := range envelope.Requirements {
		keysExactly(t, fmtIndex("requirement", i), item, schema.InnerRequirementFields)
	}
}

// TestKanbanJSON_CombinedEnvelopeInnerFeatureFields is the feature-side
// counterpart to TestKanbanJSON_CombinedEnvelopeInnerRequirementFields.
func TestKanbanJSON_CombinedEnvelopeInnerFeatureFields(t *testing.T) {
	schema := loadKanbanSchema(t)

	buf := &bytes.Buffer{}
	if err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{json: true}, kanbanSampleDeps()); err != nil {
		t.Fatalf("runKanban --json: %v", err)
	}
	var envelope struct {
		Features []map[string]json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v; raw:\n%s", err, buf.String())
	}
	if len(envelope.Features) == 0 {
		t.Fatalf("expected at least one feature in sample; got 0")
	}
	for i, item := range envelope.Features {
		keysExactly(t, fmtIndex("feature", i), item, schema.InnerFeatureFields)
	}
}

// TestKanbanJSON_RequirementsSelectorOmitsFeaturesKey verifies the
// features key is absent (not null) under --requirements — AC-7.
// Totals is scoped accordingly (no open_features key).
func TestKanbanJSON_RequirementsSelectorOmitsFeaturesKey(t *testing.T) {
	schema := loadKanbanSchema(t)

	buf := &bytes.Buffer{}
	if err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{json: true, requirements: true}, kanbanSampleDeps()); err != nil {
		t.Fatalf("runKanban: %v", err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v; raw:\n%s", err, buf.String())
	}
	keysExactly(t, "requirements-selector envelope", envelope, schema.EnvelopeRequirementsSelector)

	var totals map[string]json.RawMessage
	if err := json.Unmarshal(envelope["totals"], &totals); err != nil {
		t.Fatalf("decode totals: %v", err)
	}
	keysExactly(t, "requirements-selector totals", totals, schema.TotalsFieldsRequirementsSelector)
}

// TestKanbanJSON_FeaturesSelectorOmitsRequirementsKey is the symmetric
// selector check — AC-7.
func TestKanbanJSON_FeaturesSelectorOmitsRequirementsKey(t *testing.T) {
	schema := loadKanbanSchema(t)

	buf := &bytes.Buffer{}
	if err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{json: true, features: true}, kanbanSampleDeps()); err != nil {
		t.Fatalf("runKanban: %v", err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v; raw:\n%s", err, buf.String())
	}
	keysExactly(t, "features-selector envelope", envelope, schema.EnvelopeFeaturesSelector)

	var totals map[string]json.RawMessage
	if err := json.Unmarshal(envelope["totals"], &totals); err != nil {
		t.Fatalf("decode totals: %v", err)
	}
	keysExactly(t, "features-selector totals", totals, schema.TotalsFieldsFeaturesSelector)
}

// TestKanbanJSON_JQParseableOutput verifies that the default envelope is
// valid JSON that can be decoded — a regression guard for AC-12 which
// the manual smoke test pipes through jq. Running the decoder in-process
// is equivalent coverage without the shelling-out.
func TestKanbanJSON_JQParseableOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{json: true}, kanbanSampleDeps()); err != nil {
		t.Fatalf("runKanban: %v", err)
	}
	var anyShape interface{}
	dec := json.NewDecoder(buf)
	if err := dec.Decode(&anyShape); err != nil {
		t.Fatalf("output is not valid JSON: %v; raw:\n%s", err, buf.String())
	}
	// There must be no trailing non-whitespace — jq would reject it.
	var rest json.RawMessage
	if err := dec.Decode(&rest); err == nil {
		t.Errorf("output contains trailing content after the envelope; raw:\n%s", buf.String())
	}
}

// fmtIndex is a tiny helper that returns "context[i]" without pulling
// in fmt.Sprintf at the test site.
func fmtIndex(context string, i int) string {
	return context + "[" + itoa(i) + "]"
}

// itoa is a small stdlib-free integer-to-string for diagnostics.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
