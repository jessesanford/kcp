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

// Hub marks this version as a conversion hub.
// This allows Kubernetes to convert between different API versions
// using this version as the intermediate representation.
//
// +kubebuilder:storageversion
func (s *SyncTarget) Hub() {}

// Hub marks this version as a conversion hub.
func (s *SyncTargetList) Hub() {}

// NOTE: Conversion functions will be implemented here when additional
// API versions are added. For now, this serves as the hub version
// and establishes the pattern for future conversions.
//
// Example conversion functions that would be added for v1alpha2:
//
// func (src *v1alpha1.SyncTarget) ConvertTo(dstRaw conversion.Hub) error {
//     dst := dstRaw.(*v1alpha2.SyncTarget)
//     return Convert_v1alpha1_SyncTarget_To_v1alpha2_SyncTarget(src, dst, nil)
// }
//
// func (dst *v1alpha1.SyncTarget) ConvertFrom(srcRaw conversion.Hub) error {
//     src := srcRaw.(*v1alpha2.SyncTarget)  
//     return Convert_v1alpha2_SyncTarget_To_v1alpha1_SyncTarget(src, dst, nil)
// }

// ConversionStubs placeholder for future conversion test infrastructure.
// These will be expanded when additional API versions are introduced.
type ConversionStubs struct {
	// Future conversion test helpers will be defined here
}