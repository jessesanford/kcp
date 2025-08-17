package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	placementv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/placement/v1alpha1"
)

// Install registers the API group and adds the types to a scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(placementv1alpha1.AddToScheme(scheme))
}
