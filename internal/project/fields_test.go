package project

import (
	"errors"
	"testing"
)

func TestFindField(t *testing.T) {
	fields := []ProjectField{
		{ID: "f1", Name: "Status", DataType: "SINGLE_SELECT"},
		{ID: "f2", Name: "Target repo", DataType: "TEXT"},
	}
	if id, ok := FindField(fields, "Target repo"); !ok || id != "f2" {
		t.Errorf("FindField(Target repo) = (%q,%v), want (f2,true)", id, ok)
	}
	if _, ok := FindField(fields, "Domain"); ok {
		t.Error("FindField(Domain): expected not found")
	}
}

func TestEnsureTargetRepoField_AlreadyExists_NoCreate(t *testing.T) {
	fetch := func(string) ([]ProjectField, error) {
		return []ProjectField{{ID: "existing", Name: TargetRepoFieldName, DataType: "TEXT"}}, nil
	}
	createCalled := false
	create := func(string, string, string) (string, error) {
		createCalled = true
		return "", errors.New("should not be called")
	}

	created, id, err := EnsureTargetRepoField("PVT_1", fetch, create)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected created=false when the field already exists")
	}
	if id != "existing" {
		t.Errorf("expected the existing field id, got %q", id)
	}
	if createCalled {
		t.Error("create must not be called when the field exists")
	}
}

func TestEnsureTargetRepoField_Absent_Creates(t *testing.T) {
	fetch := func(string) ([]ProjectField, error) {
		return []ProjectField{{ID: "f1", Name: "Status"}}, nil
	}
	var gotName, gotType string
	create := func(_ string, name, dataType string) (string, error) {
		gotName, gotType = name, dataType
		return "new-field", nil
	}

	created, id, err := EnsureTargetRepoField("PVT_1", fetch, create)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created || id != "new-field" {
		t.Errorf("expected created=true id=new-field, got created=%v id=%q", created, id)
	}
	if gotName != TargetRepoFieldName || gotType != "TEXT" {
		t.Errorf("create called with (%q,%q), want (%q,TEXT)", gotName, gotType, TargetRepoFieldName)
	}
}

func TestEnsureTargetRepoField_FetchError_Propagates(t *testing.T) {
	fetch := func(string) ([]ProjectField, error) { return nil, errors.New("boom") }
	create := func(string, string, string) (string, error) { return "", nil }

	if _, _, err := EnsureTargetRepoField("PVT_1", fetch, create); err == nil {
		t.Fatal("expected fetch error to propagate")
	}
}

func TestEnsureTargetRepoField_CreateError_Propagates(t *testing.T) {
	fetch := func(string) ([]ProjectField, error) { return nil, nil }
	create := func(string, string, string) (string, error) { return "", errors.New("denied") }

	if _, _, err := EnsureTargetRepoField("PVT_1", fetch, create); err == nil {
		t.Fatal("expected create error to propagate")
	}
}
