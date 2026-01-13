package main

// TestStruct - test structure for checking Reset method generation
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
type AnotherTestStruct struct {
	ID      int
	Name    string
	Enabled bool
	Values  []int
	Config  map[string]string
	Nested  *NestedStruct
}

// NestedStruct - nested structure
type NestedStruct struct {
	Field1 string
	Field2 int
}

func (s *TestStruct) Reset() {
	if s == nil {
		return
	}
	s.Name = ""
	s.Age = 0
	s.Active = false
	s.Score = 0.0
	s.Tags = s.Tags[:0]
	clear(s.Data)
	if s.Pointer != nil {
		*s.Pointer = ""
	}
}

// Reset resets all field values of structure AnotherTestStruct to zero value
func (s *AnotherTestStruct) Reset() {
	if s == nil {
		return
	}
	s.ID = 0
	s.Name = ""
	s.Enabled = false
	s.Values = s.Values[:0]
	clear(s.Config)
	if s.Nested != nil {
		s.Nested.Reset()
	}
}

// Reset resets all field values of structure NestedStruct to zero value
func (s *NestedStruct) Reset() {
	if s == nil {
		return
	}
	s.Field1 = ""
	s.Field2 = 0
}
