// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package workqueue

import "fmt"

// IndexNotFoundError is returned when attempting to access a non-existent index
type IndexNotFoundError struct {
	IndexName string
}

// Error implements the error interface
func (e *IndexNotFoundError) Error() string {
	return fmt.Sprintf("index not found: %s", e.IndexName)
}

// IsIndexNotFound returns true if the error is an IndexNotFoundError
func IsIndexNotFound(err error) bool {
	_, ok := err.(*IndexNotFoundError)
	return ok
}

// NewIndexNotFoundError creates a new IndexNotFoundError
func NewIndexNotFoundError(indexName string) error {
	return &IndexNotFoundError{IndexName: indexName}
}

// InvalidKeyError is returned when a queue key cannot be parsed or indexed
type InvalidKeyError struct {
	Key    string
	Reason string
}

// Error implements the error interface
func (e *InvalidKeyError) Error() string {
	return fmt.Sprintf("invalid key %q: %s", e.Key, e.Reason)
}

// IsInvalidKey returns true if the error is an InvalidKeyError
func IsInvalidKey(err error) bool {
	_, ok := err.(*InvalidKeyError)
	return ok
}

// NewInvalidKeyError creates a new InvalidKeyError
func NewInvalidKeyError(key, reason string) error {
	return &InvalidKeyError{Key: key, Reason: reason}
}