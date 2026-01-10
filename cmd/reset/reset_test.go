package main

import (
	"testing"
)

func TestTestStructReset(t *testing.T) {
	// Create a test struct with non-zero values
	str := "test"
	ts := &TestStruct{
		Name:    "John",
		Age:     30,
		Active:  true,
		Score:   95.5,
		Tags:    []string{"tag1", "tag2"},
		Data:    map[string]interface{}{"key": "value"},
		Pointer: &str,
	}

	// Call Reset
	ts.Reset()

	// Check that all fields are reset to zero values
	if ts.Name != "" {
		t.Errorf("Expected Name to be empty, got %s", ts.Name)
	}
	if ts.Age != 0 {
		t.Errorf("Expected Age to be 0, got %d", ts.Age)
	}
	if ts.Active != false {
		t.Errorf("Expected Active to be false, got %t", ts.Active)
	}
	if ts.Score != 0.0 {
		t.Errorf("Expected Score to be 0.0, got %f", ts.Score)
	}
	if len(ts.Tags) != 0 {
		t.Errorf("Expected Tags to be empty, got %v", ts.Tags)
	}
	if len(ts.Data) != 0 {
		t.Errorf("Expected Data to be empty, got %v", ts.Data)
	}
	if ts.Pointer == nil {
		t.Error("Expected Pointer to not be nil")
	} else if *ts.Pointer != "" {
		t.Errorf("Expected pointed value to be empty, got %s", *ts.Pointer)
	}
}

func TestAnotherTestStructReset(t *testing.T) {
	// Create a nested struct
	nested := &NestedStruct{
		Field1: "nested",
		Field2: 42,
	}

	// Create a test struct with non-zero values
	ats := &AnotherTestStruct{
		ID:      123,
		Name:    "Jane",
		Enabled: true,
		Values:  []int{1, 2, 3},
		Config:  map[string]string{"setting": "on"},
		Nested:  nested,
	}

	// Call Reset
	ats.Reset()

	// Check that all fields are reset to zero values
	if ats.ID != 0 {
		t.Errorf("Expected ID to be 0, got %d", ats.ID)
	}
	if ats.Name != "" {
		t.Errorf("Expected Name to be empty, got %s", ats.Name)
	}
	if ats.Enabled != false {
		t.Errorf("Expected Enabled to be false, got %t", ats.Enabled)
	}
	if len(ats.Values) != 0 {
		t.Errorf("Expected Values to be empty, got %v", ats.Values)
	}
	if len(ats.Config) != 0 {
		t.Errorf("Expected Config to be empty, got %v", ats.Config)
	}

	// Check that nested struct was also reset
	if ats.Nested != nil {
		if ats.Nested.Field1 != "" {
			t.Errorf("Expected Nested.Field1 to be empty, got %s", ats.Nested.Field1)
		}
		if ats.Nested.Field2 != 0 {
			t.Errorf("Expected Nested.Field2 to be 0, got %d", ats.Nested.Field2)
		}
	}
}

func TestNilReceiver(t *testing.T) {
	// Test that calling Reset on a nil receiver doesn't panic
	var ts *TestStruct
	ts.Reset() // Should not panic

	var ats *AnotherTestStruct
	ats.Reset() // Should not panic

	var ns *NestedStruct
	ns.Reset() // Should not panic
}
