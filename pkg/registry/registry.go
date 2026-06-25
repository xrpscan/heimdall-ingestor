package registry

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

// Registry can be used to register services (Close-ables). All registered services are closed once
// root context expires (as accepted by the New function).
//
// Note that Registry's zero-value does not work as intended.
// New must be called to create a usable Registry.
type Registry struct {
	logger Logger

	mutex    sync.RWMutex
	services map[string]Closer
	keyOrder []string
}

// New returns a new Registry instance. Note that Registry's zero-value does not work as intended.
// New must be called to create a usable Registry.
//
// If logging is not required, nil can be passed for the logger.
func New(logger Logger) *Registry {
	// Default to no-op logger.
	if logger == nil {
		logger = noopLogger{}
	}

	// Instantiate registry.
	registry := &Registry{
		logger:   logger,
		mutex:    sync.RWMutex{},
		services: map[string]Closer{},
		keyOrder: []string{},
	}

	return registry
}

// Register a new service. When the root context expires or MustCloseAll is called, all registered
// services will be closed.
func (r *Registry) Register(name string, svc Closer) {
	// Lock for registering the service.
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Panic if already registered. We must not overwrite.
	if _, exists := r.services[name]; exists {
		panic("service with name " + name + " is already registered")
	}

	// Register.
	r.services[name] = svc
	r.keyOrder = append(r.keyOrder, name)
}

// RegisterWithFunc is the same as Register but allows users to provide a CloserFunc instead of a
// Closer interface implementation.
//
// The call essentially becomes:
//
//	RegisterWithFunc("my-service", func(ctx context.Context) error { ... })
//
// Instead of:
//
//	Register("my-service", CloserFunc(func(ctx context.Context) error { ... }))
func (r *Registry) RegisterWithFunc(name string, closeFn CloserFunc) {
	r.Register(name, closeFn)
}

// MustCloseAll closes all registered services. If any service fails to Close, its error is
// collected and the method moves to the next service. At the end, if there have been any errors,
// the method panics.
func (r *Registry) MustCloseAll() {
	r.mutex.Lock()
	mapClone := maps.Clone(r.services)
	arrClone := slices.Clone(r.keyOrder)

	// Reset data so any superfluous calls are no-ops.
	r.services = map[string]Closer{}
	r.keyOrder = []string{}
	r.mutex.Unlock()

	// Context for all the svc.Close calls.
	ctx, cancel := contextWithOneMinute(context.Background())
	defer cancel()

	// For collecting closure errors.
	var errs []error

	// Close all services in reverse order.
	for i := len(arrClone) - 1; i >= 0; i-- {
		name, svc := arrClone[i], mapClone[arrClone[i]]

		// Defend against nil-pointer panics.
		if svc == nil {
			continue
		}

		// Collect errors to call panic() later, if needed.
		if err := svc.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to close %s gracefully: %w", name, err))
		} else {
			r.logger.Info("successfully closed " + name)
		}
	}

	// If any errors occurred, panic.
	if len(errs) != 0 {
		panic("failed to close all services gracefully: " + errors.Join(errs...).Error())
	}
}
