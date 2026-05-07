package injection

import (
	"context"
	"encoding/json"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const originalCommandAnnotationPrefix = "chaos.operatorchaos.io/original-command-"
const crashContainerAnnotation = "chaos.operatorchaos.io/crash-container-name"

// crashLoopCommand is the nonexistent binary used to trigger CrashLoopBackOff.
var crashLoopCommand = []string{"/chaos-nonexistent-binary"}

// CrashLoopInjector patches a Deployment template's container command to a
// nonexistent binary, causing CrashLoopBackOff. This tests whether operators
// detect degraded pods.
type CrashLoopInjector struct {
	client client.Client
}

func NewCrashLoopInjector(c client.Client) *CrashLoopInjector {
	return &CrashLoopInjector{client: c}
}

func (cl *CrashLoopInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateCrashLoopParams(spec)
}

func (cl *CrashLoopInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	name := spec.Parameters["name"]
	containerName := spec.Parameters["containerName"]
	key := types.NamespacedName{Name: name, Namespace: namespace}

	deploy := &appsv1.Deployment{}
	if err := cl.client.Get(ctx, key, deploy); err != nil {
		return nil, nil, fmt.Errorf("getting Deployment %s: %w", key, err)
	}

	// Find the target container
	containerIdx, err := findContainer(deploy, containerName)
	if err != nil {
		return nil, nil, err
	}

	// Resolve the actual container name (may have defaulted to first)
	targetContainer := deploy.Spec.Template.Spec.Containers[containerIdx].Name

	// Store original command as JSON in annotation, keyed by container name
	originalCmd := deploy.Spec.Template.Spec.Containers[containerIdx].Command
	var cmdJSON string
	if len(originalCmd) == 0 {
		cmdJSON = "null"
	} else {
		data, err := json.Marshal(originalCmd)
		if err != nil {
			return nil, nil, fmt.Errorf("marshaling original command: %w", err)
		}
		cmdJSON = string(data)
	}

	annotationKey := originalCommandAnnotationPrefix + targetContainer

	patch := client.MergeFrom(deploy.DeepCopy())
	if deploy.Annotations == nil {
		deploy.Annotations = make(map[string]string)
	}
	deploy.Annotations[annotationKey] = cmdJSON
	deploy.Annotations[crashContainerAnnotation] = targetContainer
	deploy.Spec.Template.Spec.Containers[containerIdx].Command = crashLoopCommand

	if err := cl.client.Patch(ctx, deploy, patch); err != nil {
		return nil, nil, fmt.Errorf("patching Deployment %s to crash loop: %w", key, err)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.CrashLoopInject, key.String(), "command-replaced",
			map[string]string{
				"namespace":       namespace,
				"deployment":      name,
				"container":       targetContainer,
				"originalCommand": cmdJSON,
			}),
	}

	cleanup := func(ctx context.Context) error {
		return cl.restore(ctx, name, namespace)
	}

	return cleanup, events, nil
}

func (cl *CrashLoopInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	return cl.restore(ctx, spec.Parameters["name"], namespace)
}

func (cl *CrashLoopInjector) restore(ctx context.Context, name, namespace string) error {
	key := types.NamespacedName{Name: name, Namespace: namespace}

	deploy := &appsv1.Deployment{}
	if err := cl.client.Get(ctx, key, deploy); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting Deployment %s for restore: %w", key, err)
	}

	// Read the stored container name
	storedContainer, ok := deploy.Annotations[crashContainerAnnotation]
	if !ok {
		return nil
	}

	annotationKey := originalCommandAnnotationPrefix + storedContainer
	cmdJSON, ok := deploy.Annotations[annotationKey]
	if !ok {
		return nil
	}

	// Use findContainer to locate the target container by name
	containerIdx, err := findContainer(deploy, storedContainer)
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}

	patch := client.MergeFrom(deploy.DeepCopy())

	if cmdJSON == "null" {
		// Original container used ENTRYPOINT (no command override)
		deploy.Spec.Template.Spec.Containers[containerIdx].Command = nil
	} else {
		var originalCmd []string
		if err := json.Unmarshal([]byte(cmdJSON), &originalCmd); err != nil {
			return fmt.Errorf("parsing original command annotation %q: %w", cmdJSON, err)
		}
		deploy.Spec.Template.Spec.Containers[containerIdx].Command = originalCmd
	}

	delete(deploy.Annotations, annotationKey)
	delete(deploy.Annotations, crashContainerAnnotation)

	if err := cl.client.Patch(ctx, deploy, patch); err != nil {
		return fmt.Errorf("restoring Deployment %s command: %w", key, err)
	}

	return nil
}

// findContainer locates a container by name in the Deployment template.
// If containerName is empty, it defaults to the first container (index 0).
func findContainer(deploy *appsv1.Deployment, containerName string) (int, error) {
	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		return -1, fmt.Errorf("deployment %s/%s has no containers", deploy.Namespace, deploy.Name)
	}

	if containerName == "" {
		return 0, nil
	}

	for i, c := range containers {
		if c.Name == containerName {
			return i, nil
		}
	}

	return -1, fmt.Errorf("container %q not found in Deployment %s/%s", containerName, deploy.Namespace, deploy.Name)
}
