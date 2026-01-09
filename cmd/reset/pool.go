package reset

import (
	"sync"
)

// Resetter interface defines the Reset method that objects must implement
// This allows the pool to work with any type that can be reset
type Resetter interface {
	Reset()
}

// Pool is a generic pool of objects that implement the Resetter interface
// It uses sync.Pool internally for efficient object management
type Pool[T Resetter] struct {
	pool sync.Pool
}

// New creates and returns a new Pool instance
// The sync.Pool's New function returns a zero value of type T when the pool is empty
func New[T Resetter]() *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				// var zero T returns the zero value for type T
				// For pointers, this will be nil
				var zero T
				return zero
			},
		},
	}
}

// Get retrieves an object from the pool
// If the pool is empty, it returns the zero value of type T
func (p *Pool[T]) Get() T {
	obj := p.pool.Get()
	if obj != nil {
		return obj.(T)
	}
	// Return zero value if pool is empty and New function returned nil
	var zero T
	return zero
}

// Put places an object back into the pool
// It calls Reset() on the object before putting it back
func (p *Pool[T]) Put(obj T) {
	obj.Reset()
	p.pool.Put(obj)
}
