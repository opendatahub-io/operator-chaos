package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is the API group and version for ChaosExperiment resources.
	GroupVersion = schema.GroupVersion{Group: "chaos.operatorchaos.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add Go types to the GroupVersionResource scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion,
		&ChaosExperiment{},
		&ChaosExperimentList{},
	)
	return nil
}
