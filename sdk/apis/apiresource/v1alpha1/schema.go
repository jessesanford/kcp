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

package v1alpha1

import (
	"encoding/json"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SchemaIntersection represents the result of schema intersection analysis
type SchemaIntersection struct {
	// CommonProperties are properties that exist in all schemas
	CommonProperties map[string]*apiextensionsv1.JSONSchemaProps `json:"commonProperties,omitempty"`

	// RequiredFields are fields required by all schemas
	RequiredFields []string `json:"requiredFields,omitempty"`

	// ConflictingFields are fields that have different definitions across schemas
	ConflictingFields []SchemaConflict `json:"conflictingFields,omitempty"`
}

// SchemaConflict represents a conflict between schema definitions
type SchemaConflict struct {
	// FieldPath is the path to the conflicting field
	FieldPath string `json:"fieldPath"`

	// ConflictType describes the type of conflict
	ConflictType string `json:"conflictType"`

	// Schemas maps location names to their schema definitions for this field
	Schemas map[string]*apiextensionsv1.JSONSchemaProps `json:"schemas,omitempty"`
}

// ExtractCommonSchema analyzes multiple CRD definitions and extracts the common schema
// elements that are compatible across all of them. This is used for API negotiation
// to determine what fields can be safely used across different sync targets.
func ExtractCommonSchema(crds []apiextensionsv1.CustomResourceDefinition) (*runtime.RawExtension, error) {
	if len(crds) == 0 {
		return nil, fmt.Errorf("no CRDs provided for schema extraction")
	}

	// Start with the first CRD's schema as the baseline
	baseCRD := crds[0]
	if len(baseCRD.Spec.Versions) == 0 {
		return nil, fmt.Errorf("CRD %s has no versions defined", baseCRD.Name)
	}

	// Use the storage version as the baseline
	var baseSchema *apiextensionsv1.JSONSchemaProps
	for _, version := range baseCRD.Spec.Versions {
		if version.Storage {
			if version.Schema != nil && version.Schema.OpenAPIV3Schema != nil {
				baseSchema = version.Schema.OpenAPIV3Schema
			}
			break
		}
	}

	if baseSchema == nil {
		return nil, fmt.Errorf("no schema found in storage version of CRD %s", baseCRD.Name)
	}

	// Extract intersection of all schemas
	intersection, err := intersectSchemas(crds)
	if err != nil {
		return nil, fmt.Errorf("failed to intersect schemas: %w", err)
	}

	// Convert intersection back to JSONSchemaProps
	commonSchema := &apiextensionsv1.JSONSchemaProps{
		Type:                 baseSchema.Type,
		Properties:           intersection.CommonProperties,
		Required:             intersection.RequiredFields,
		AdditionalProperties: baseSchema.AdditionalProperties,
	}

	// Serialize to RawExtension
	schemaBytes, err := json.Marshal(commonSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal common schema: %w", err)
	}

	return &runtime.RawExtension{
		Raw: schemaBytes,
	}, nil
}

// intersectSchemas finds the intersection of schemas across multiple CRDs
func intersectSchemas(crds []apiextensionsv1.CustomResourceDefinition) (*SchemaIntersection, error) {
	intersection := &SchemaIntersection{
		CommonProperties:  make(map[string]*apiextensionsv1.JSONSchemaProps),
		ConflictingFields: []SchemaConflict{},
	}

	if len(crds) == 0 {
		return intersection, nil
	}

	// Collect all schemas from storage versions
	schemas := make([]*apiextensionsv1.JSONSchemaProps, 0, len(crds))
	locationNames := make([]string, 0, len(crds))

	for _, crd := range crds {
		for _, version := range crd.Spec.Versions {
			if version.Storage && version.Schema != nil && version.Schema.OpenAPIV3Schema != nil {
				schemas = append(schemas, version.Schema.OpenAPIV3Schema)
				locationNames = append(locationNames, crd.Name) // Use CRD name as location identifier
				break
			}
		}
	}

	if len(schemas) == 0 {
		return intersection, fmt.Errorf("no valid schemas found in storage versions")
	}

	// Find properties that exist in all schemas
	baseProperties := schemas[0].Properties
	if baseProperties == nil {
		return intersection, nil
	}

	for propName, baseProp := range baseProperties {
		isCommon := true
		var conflictSchemas map[string]*apiextensionsv1.JSONSchemaProps

		// Check if this property exists in all other schemas with compatible types
		for i := 1; i < len(schemas); i++ {
			if schemas[i].Properties == nil {
				isCommon = false
				break
			}

			otherProp, exists := schemas[i].Properties[propName]
			if !exists {
				isCommon = false
				break
			}

			// Check for type compatibility
			if !arePropsCompatible(baseProp, &otherProp) {
				isCommon = false
				// Track this as a conflict
				if conflictSchemas == nil {
					conflictSchemas = make(map[string]*apiextensionsv1.JSONSchemaProps)
					conflictSchemas[locationNames[0]] = baseProp
				}
				conflictSchemas[locationNames[i]] = &otherProp
			}
		}

		if isCommon {
			intersection.CommonProperties[propName] = baseProp
		} else if conflictSchemas != nil {
			intersection.ConflictingFields = append(intersection.ConflictingFields, SchemaConflict{
				FieldPath:    fmt.Sprintf(".%s", propName),
				ConflictType: "type_mismatch",
				Schemas:      conflictSchemas,
			})
		}
	}

	// Find fields that are required in all schemas
	intersection.RequiredFields = findCommonRequiredFields(schemas)

	return intersection, nil
}

// arePropsCompatible checks if two JSONSchemaProps are compatible
func arePropsCompatible(prop1, prop2 *apiextensionsv1.JSONSchemaProps) bool {
	// Basic type compatibility
	if prop1.Type != prop2.Type {
		return false
	}

	// Format compatibility
	if prop1.Format != nil && prop2.Format != nil && *prop1.Format != *prop2.Format {
		return false
	}

	// For now, we'll consider them compatible if types match
	// More sophisticated compatibility checks can be added here
	return true
}

// findCommonRequiredFields finds fields that are required in all schemas
func findCommonRequiredFields(schemas []*apiextensionsv1.JSONSchemaProps) []string {
	if len(schemas) == 0 {
		return nil
	}

	commonRequired := make([]string, 0)
	baseRequired := schemas[0].Required

	for _, field := range baseRequired {
		isCommon := true

		// Check if this field is required in all other schemas
		for i := 1; i < len(schemas); i++ {
			found := false
			for _, reqField := range schemas[i].Required {
				if reqField == field {
					found = true
					break
				}
			}
			if !found {
				isCommon = false
				break
			}
		}

		if isCommon {
			commonRequired = append(commonRequired, field)
		}
	}

	return commonRequired
}

// ValidateAgainstSchema validates an object against the provided common schema.
// This can be used to verify that a workload conforms to the negotiated API schema.
func ValidateAgainstSchema(obj runtime.Object, schema *runtime.RawExtension) error {
	if schema == nil {
		return nil // No schema to validate against
	}

	// Parse the schema
	var schemaProps apiextensionsv1.JSONSchemaProps
	if err := json.Unmarshal(schema.Raw, &schemaProps); err != nil {
		return fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	// Convert object to unstructured for validation
	objBytes, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object for validation: %w", err)
	}

	var objMap map[string]interface{}
	if err := json.Unmarshal(objBytes, &objMap); err != nil {
		return fmt.Errorf("failed to unmarshal object map: %w", err)
	}

	// Validate required fields
	return validateRequiredFields(objMap, schemaProps.Required)
}

// validateRequiredFields checks that all required fields are present in the object
func validateRequiredFields(objMap map[string]interface{}, required []string) error {
	for _, field := range required {
		if _, exists := objMap[field]; !exists {
			return fmt.Errorf("required field %s is missing", field)
		}
	}
	return nil
}

// GetSchemaConflicts returns any schema conflicts found during negotiation
func (n *NegotiatedAPIResource) GetSchemaConflicts() []SchemaConflict {
	if n.Status.Phase != NegotiationIncompatible {
		return nil
	}

	// Extract conflicts from status (this would be populated by the controller)
	// For now, return empty slice as conflicts would be stored differently
	return []SchemaConflict{}
}
