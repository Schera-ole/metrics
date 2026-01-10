package main

import (
	"sync"
)

// Resetter interface defines the Reset method that objects must implement
type Resetter interface {
	Reset()
}

// Pool is a generic pool (sync.Pool) of objects that implement the Resetter interface
type Pool[T Resetter] struct {
	pool sync.Pool
}

// Creates and returns a new Pool instance. sync.Pool's New returns a zero value of type T when the pool is empty
func New[T Resetter]() *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				var zero T
				return zero
			},
		},
	}
}

// Get retrieves an object from the pool
func (p *Pool[T]) Get() T {
	obj := p.pool.Get()
	if obj != nil {
		return obj.(T)
	}
	// Return zero value if pool is empty
	var zero T
	return zero
}

// Put places an object back into the pool
func (p *Pool[T]) Put(obj T) {
	obj.Reset()
	p.pool.Put(obj)
}
