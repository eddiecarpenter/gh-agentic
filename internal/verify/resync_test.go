package verify

import (
	"fmt"
	"strings"
	"testing"
)

// ── resyncProjectItemStatuses — item processing loop ─────────────────────────
// These tests exercise the body of the for-range loop that was untested.

// itemsJSON builds a GraphQL response containing a single project item.
// fieldID is the status field ID to embed in fieldValues.
func itemsJSON(itemID, state, currentStatus, fieldID string, labels []string, issueNumber int) string {
	labelNodes := ""
	for i, l := range labels {
		if i > 0 {
			labelNodes += ","
		}
		labelNodes += fmt.Sprintf(`{"name":%q}`, l)
	}
	fieldValueNode := ""
	if currentStatus != "" {
		fieldValueNode = fmt.Sprintf(`{"field":{"id":%q},"name":%q}`, fieldID, currentStatus)
	}
	return fmt.Sprintf(
		`{"data":{"node":{"items":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[{"id":%q,"content":{"number":%d,"repository":{"nameWithOwner":"owner/repo"},"state":%q,"labels":{"nodes":[%s]}},"fieldValues":{"nodes":[%s]}}]}}}}`,
		itemID, issueNumber, state, labelNodes, fieldValueNode)
}

// ── status not in optionMap → skip (continue) ────────────────────────────────

func TestResyncProjectItemStatuses_StatusNotInOptionMap_Skips(t *testing.T) {
	const fieldID = "FIELD_X"
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			// fetchStatusOptionMap — returns only "Done" option
			return "OPT_DONE|Done\n", nil
		default:
			// fetchAllProjectItems — item has "in-development" label → resolves to "In Development"
			// "In Development" is not in optionMap → skip
			return itemsJSON("ITEM_1", "OPEN", "", fieldID, []string{"in-development"}, 0), nil
		}
	}

	updated, correct, err := resyncProjectItemStatuses("PROJ_1", fieldID, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != 0 || correct != 0 {
		t.Errorf("expected updated=0 correct=0, got updated=%d correct=%d", updated, correct)
	}
}

// ── item already correct → correct++ ─────────────────────────────────────────

func TestResyncProjectItemStatuses_ItemAlreadyCorrect_IncreasesCorrect(t *testing.T) {
	const fieldID = "FIELD_X"
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return "OPT_BL|Backlog\n", nil
		default:
			// Item: OPEN, "backlog" label → resolves to "Backlog"; currentStatus="Backlog" ✓
			return itemsJSON("ITEM_1", "OPEN", "Backlog", fieldID, []string{"backlog"}, 0), nil
		}
	}

	updated, correct, err := resyncProjectItemStatuses("PROJ_1", fieldID, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if correct != 1 || updated != 0 {
		t.Errorf("expected correct=1 updated=0, got correct=%d updated=%d", correct, updated)
	}
}

// ── status update succeeds → updated++ ───────────────────────────────────────

func TestResyncProjectItemStatuses_StatusUpdateSucceeds_IncreasesUpdated(t *testing.T) {
	const fieldID = "FIELD_X"
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return "OPT_BL|Backlog\n", nil
		case 2:
			// Item: OPEN, "backlog" label → "Backlog"; currentStatus="In Development" (wrong)
			return itemsJSON("ITEM_1", "OPEN", "In Development", fieldID, []string{"backlog"}, 0), nil
		default:
			// Mutation to update status → succeeds
			return `{"data":{"updateProjectV2ItemFieldValue":{"clientMutationId":null}}}`, nil
		}
	}

	updated, correct, err := resyncProjectItemStatuses("PROJ_1", fieldID, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != 1 || correct != 0 {
		t.Errorf("expected updated=1 correct=0, got updated=%d correct=%d", updated, correct)
	}
}

// ── status update fails → returns error ──────────────────────────────────────

func TestResyncProjectItemStatuses_StatusUpdateFails_ReturnsError(t *testing.T) {
	const fieldID = "FIELD_X"
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return "OPT_BL|Backlog\n", nil
		case 2:
			return itemsJSON("ITEM_1", "OPEN", "In Development", fieldID, []string{"backlog"}, 0), nil
		default:
			return "", fmt.Errorf("graphql: mutation failed")
		}
	}

	_, _, err := resyncProjectItemStatuses("PROJ_1", fieldID, fakeRun)
	if err == nil {
		t.Fatal("expected error for mutation failure, got nil")
	}
	if !strings.Contains(err.Error(), "updating item") {
		t.Errorf("expected 'updating item' in error, got: %v", err)
	}
}

// ── CLOSED item with stale labels → label fix succeeds ───────────────────────

func TestResyncProjectItemStatuses_ClosedItem_LabelFixSucceeds(t *testing.T) {
	const fieldID = "FIELD_X"
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return "OPT_DONE|Done\n", nil
		case 2:
			// CLOSED item with "in-development" stale label, currentStatus="Done" (already correct)
			return itemsJSON("ITEM_1", "CLOSED", "Done", fieldID, []string{"in-development"}, 5), nil
		default:
			// "gh issue edit" to remove stale label + add "done"
			return "", nil
		}
	}

	updated, correct, err := resyncProjectItemStatuses("PROJ_1", fieldID, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// CLOSED resolves to "Done" == currentStatus → correct, not updated
	if correct != 1 || updated != 0 {
		t.Errorf("expected correct=1 updated=0, got correct=%d updated=%d", correct, updated)
	}
}

// ── CLOSED item — label fix fails → returns error ────────────────────────────

func TestResyncProjectItemStatuses_ClosedItem_LabelFixFails_ReturnsError(t *testing.T) {
	const fieldID = "FIELD_X"
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return "OPT_DONE|Done\n", nil
		case 2:
			return itemsJSON("ITEM_1", "CLOSED", "Done", fieldID, []string{"in-development"}, 5), nil
		default:
			return "", fmt.Errorf("issue edit: permission denied")
		}
	}

	_, _, err := resyncProjectItemStatuses("PROJ_1", fieldID, fakeRun)
	if err == nil {
		t.Fatal("expected error when label fix fails, got nil")
	}
	if !strings.Contains(err.Error(), "fixing labels on issue") {
		t.Errorf("expected 'fixing labels on issue' in error, got: %v", err)
	}
}

// ── CLOSED item — "done" label already present, no stale labels → no edit ────

func TestResyncProjectItemStatuses_ClosedItem_AlreadyDoneLabel_NoEdit(t *testing.T) {
	const fieldID = "FIELD_X"
	callCount := 0
	fakeRun := func(name string, args ...string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			return "OPT_DONE|Done\n", nil
		case 2:
			// CLOSED, "done" label present (needsDone=false), no stale labels → skip edit
			return itemsJSON("ITEM_1", "CLOSED", "Done", fieldID, []string{"done"}, 5), nil
		default:
			// If edit is wrongly called, fail the test.
			t.Errorf("unexpected extra run call %d: %s %v", callCount, name, args)
			return "", nil
		}
	}

	updated, correct, err := resyncProjectItemStatuses("PROJ_1", fieldID, fakeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if correct != 1 || updated != 0 {
		t.Errorf("expected correct=1 updated=0, got correct=%d updated=%d", correct, updated)
	}
}
