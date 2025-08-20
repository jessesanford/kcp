/*
Copyright 2024 The KCP Authors.

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

package tmc

import (
	"fmt"
)

// TMCErrorType defines the types of errors that can occur in TMC operations
type TMCErrorType string

const (
	TMCErrorTypeResourceValidation TMCErrorType = "ResourceValidation"
	TMCErrorTypeSyncFailure        TMCErrorType = "SyncFailure"
	TMCErrorTypeClusterUnreachable TMCErrorType = "ClusterUnreachable"
)

// TMCError represents a structured error for TMC operations
type TMCError struct {
	Type      TMCErrorType
	Component string
	Operation string
	Message   string
	Cause     error
}

// NewTMCError creates a new TMCError with the given type, component, and operation
func NewTMCError(errType TMCErrorType, component, operation string) *TMCError {
	return &TMCError{
		Type:      errType,
		Component: component,
		Operation: operation,
	}
}

// WithMessage adds a message to the TMC error
func (e *TMCError) WithMessage(message string) *TMCError {
	e.Message = message
	return e
}

// WithCause adds a cause error to the TMC error
func (e *TMCError) WithCause(cause error) *TMCError {
	e.Cause = cause
	return e
}

// Error implements the error interface
func (e *TMCError) Error() string {
	msg := fmt.Sprintf("TMC %s error in %s.%s", e.Type, e.Component, e.Operation)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	if e.Cause != nil {
		msg += " (caused by: " + e.Cause.Error() + ")"
	}
	return msg
}

// Unwrap returns the underlying cause error
func (e *TMCError) Unwrap() error {
	return e.Cause
}