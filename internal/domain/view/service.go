package view

import "context"

// Service is the view aggregate.
//
// It carries no ports yet. Components is introspection over the catalog
// embedded in this package (see catalog.go), which needs nothing injected to
// serve; every method that touches a stored View — create, read, render — is
// Task 7's, once the collection domain exists for a View to be persisted
// through.
type Service struct{}

// Deps is what the service is built from. It is empty for the same reason
// Service carries no fields: nothing this task adds needs a port. It exists
// so a caller already wires NewService(Deps{}) the way every other domain
// does, instead of a bespoke zero-argument constructor Task 7 would have to
// change every call site to widen.
type Deps struct{}

// NewService wires the service over its ports.
func NewService(Deps) *Service {
	return &Service{}
}

// Components returns every component the design system publishes: the
// introspection an agent calls before composing a screen, over the same
// catalog frontend/scripts/gen-components.mjs generated for this task.
func (*Service) Components(context.Context) ([]ComponentSpec, error) {
	return Catalog(), nil
}
