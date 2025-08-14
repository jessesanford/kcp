/*
Copyright 2024 The KCP Authors.
*/

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion is the API group and version for TMC APIs
var GroupVersion = schema.GroupVersion{Group: "tmc.kcp.io", Version: "v1alpha1"}
