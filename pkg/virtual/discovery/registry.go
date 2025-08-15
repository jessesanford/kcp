/*
Copyright 2023 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package discovery

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages discovery providers and provides a centralized way to access them.
// It supports dynamic registration and unregistration of providers, default provider
// selection, and proper lifecycle management for all registered providers.
type Registry struct {
	mu           sync.RWMutex
	providers    map[string]Provider
	defaultName  string
}

// NewRegistry creates a new provider registry.
// The registry starts empty and providers must be registered before use.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry with the given name.
// Returns an error if a provider with the same name is already registered.
// The first provider registered automatically becomes the default.
func (r *Registry) Register(name string, provider Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	r.providers[name] = provider

	// Set as default if this is the first provider
	if len(r.providers) == 1 {
		r.defaultName = name
	}

	return nil
}

// Unregister removes a provider from the registry.
// Returns an error if the provider is not found.
// If the removed provider was the default, a new default is automatically selected.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	delete(r.providers, name)

	// Clear default if it was the removed provider
	if r.defaultName == name {
		r.defaultName = ""
		// Set new default if providers remain
		for providerName := range r.providers {
			r.defaultName = providerName
			break
		}
	}

	return nil
}

// Get retrieves a provider by name.
// Returns an error if the provider is not found.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// GetDefault retrieves the current default provider.
// Returns an error if no default provider is set.
func (r *Registry) GetDefault() (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultName == "" {
		return nil, fmt.Errorf("no default provider set")
	}

	return r.providers[r.defaultName], nil
}

// SetDefault sets the default provider by name.
// Returns an error if the provider is not found in the registry.
func (r *Registry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	r.defaultName = name
	return nil
}

// List returns the names of all registered providers.
// The returned slice is a copy and can be safely modified.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}

// Close gracefully shuts down all registered providers.
// It calls Close() on each provider and returns the last error encountered.
// After closing, the registry is reset to an empty state.
func (r *Registry) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for name, provider := range r.providers {
		if err := provider.Close(ctx); err != nil {
			lastErr = fmt.Errorf("failed to close provider %s: %w", name, err)
		}
	}

	// Reset registry state after closing all providers
	r.providers = make(map[string]Provider)
	r.defaultName = ""

	return lastErr
}

// DefaultName returns the name of the current default provider.
// Returns an empty string if no default provider is set.
func (r *Registry) DefaultName() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.defaultName
}

// Count returns the number of registered providers.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.providers)
}

// Has checks if a provider with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.providers[name]
	return exists
}