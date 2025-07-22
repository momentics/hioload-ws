// File: api/context_factory.go
package api

// ContextFactory способен создавать Context, реализация передаётся из фасада.
type ContextFactory interface {
	NewContext() Context
}
