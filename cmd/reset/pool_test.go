package main

import (
	"fmt"
	"testing"
)

// TestPool show basic usage of Pool
func TestPool(t *testing.T) {
	pool := New[*TestStruct]()

	// Get an object from the pool
	obj := pool.Get()
	if obj == nil {
		t.Log("Creating new TestStruct")
		obj = &TestStruct{}
	}

	obj.Name = "Test"
	obj.Age = 30
	t.Logf("Object: %+v", obj)

	// Put the object back
	pool.Put(obj)

	// Get object from the pool again
	obj2 := pool.Get()
	t.Logf("Object from pool: %+v", obj2)

	// Check that the object was reset
	if obj2.Name != "" || obj2.Age != 0 {
		t.Errorf("Expected reset object, got Name=%s, Age=%d", obj2.Name, obj2.Age)
	}
}

// Example usage of the Pool
func ExamplePool() {
	pool := New[*TestStruct]()

	// Get an object from the pool
	obj := pool.Get()
	if obj == nil {
		obj = &TestStruct{}
	}

	obj.Name = "Test"
	obj.Age = 30
	fmt.Printf("Object: %+v\n", obj)

	// Put the object back
	pool.Put(obj)

	// Get object from the pool again
	obj2 := pool.Get()
	fmt.Printf("Object from pool: %+v\n", obj2)

	// Output:
	// Object: &{Name:Test Age:30 Active:false Score:0 Tags:[] Data:map[] Pointer:<nil>}
	// Object from pool: &{Name: Age:0 Active:false Score:0 Tags:[] Data:map[] Pointer:<nil>}
}
