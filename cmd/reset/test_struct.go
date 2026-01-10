package main

// TestStruct - test structure for checking Reset method generation
// generate:reset
type TestStruct struct {
	Name    string
	Age     int
	Active  bool
	Score   float64
	Tags    []string
	Data    map[string]interface{}
	Pointer *string
}

// AnotherTestStruct - another test structure for checking Reset method generation
// generate:reset
type AnotherTestStruct struct {
	ID      int
	Name    string
	Enabled bool
	Values  []int
	Config  map[string]string
	Nested  *NestedStruct
}

// NestedStruct - nested structure
// generate:reset
type NestedStruct struct {
	Field1 string
	Field2 int
}
