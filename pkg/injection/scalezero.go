package injection

import (
	"context"
	"fmt"
	"strconv"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const originalReplicasAnnotation = "chaos.operatorchaos.io/original-replicas"

type ScaleZeroInjector struct {
	client client.Client
}

func NewScaleZeroInjector(c client.Client) *ScaleZeroInjector {
	return &ScaleZeroInjector{client: c}
}

func (s *ScaleZeroInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateScaleZeroParams(spec)
}

func (s *ScaleZeroInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	name := spec.Parameters["name"]
	key := types.NamespacedName{Name: name, Namespace: namespace}

	deploy := &appsv1.Deployment{}
	if err := s.client.Get(ctx, key, deploy); err != nil {
		return nil, nil, fmt.Errorf("getting Deployment %s: %w", key, err)
	}

	var currentReplicas int32
	if deploy.Spec.Replicas != nil {
		currentReplicas = *deploy.Spec.Replicas
	} else {
		currentReplicas = 1
	}

	// Store original replicas in annotation
	patch := client.MergeFrom(deploy.DeepCopy())
	if deploy.Annotations == nil {
		deploy.Annotations = make(map[string]string)
	}
	deploy.Annotations[originalReplicasAnnotation] = strconv.Itoa(int(currentReplicas))
	zero := int32(0)
	deploy.Spec.Replicas = &zero

	if err := s.client.Patch(ctx, deploy, patch); err != nil {
		return nil, nil, fmt.Errorf("patching Deployment %s to scale zero: %w", key, err)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.DeploymentScaleZero, key.String(), "scaled-to-zero",
			map[string]string{
				"namespace":        namespace,
				"deployment":       name,
				"originalReplicas": strconv.Itoa(int(currentReplicas)),
			}),
	}

	cleanup := func(ctx context.Context) error {
		return s.restore(ctx, name, namespace)
	}

	return cleanup, events, nil
}

func (s *ScaleZeroInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	return s.restore(ctx, spec.Parameters["name"], namespace)
}

func (s *ScaleZeroInjector) restore(ctx context.Context, name, namespace string) error {
	key := types.NamespacedName{Name: name, Namespace: namespace}

	deploy := &appsv1.Deployment{}
	if err := s.client.Get(ctx, key, deploy); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting Deployment %s for restore: %w", key, err)
	}

	replicaStr, ok := deploy.Annotations[originalReplicasAnnotation]
	if !ok {
		return nil
	}

	original, err := strconv.Atoi(replicaStr)
	if err != nil {
		return fmt.Errorf("parsing original replicas annotation %q: %w", replicaStr, err)
	}

	patch := client.MergeFrom(deploy.DeepCopy())
	replicas := int32(original)
	deploy.Spec.Replicas = &replicas
	delete(deploy.Annotations, originalReplicasAnnotation)

	if err := s.client.Patch(ctx, deploy, patch); err != nil {
		return fmt.Errorf("restoring Deployment %s replicas: %w", key, err)
	}

	return nil
}
