package injection

import (
	"context"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	originalImageAnnotationPrefix = "chaos.operatorchaos.io/original-image-"
	corruptContainerAnnotation    = "chaos.operatorchaos.io/corrupt-container-name"
	defaultCorruptImage           = "registry.invalid/nonexistent:chaos"
)

type ImageCorruptInjector struct {
	client client.Client
}

func NewImageCorruptInjector(c client.Client) *ImageCorruptInjector {
	return &ImageCorruptInjector{client: c}
}

func (i *ImageCorruptInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateImageCorruptParams(spec)
}

func (i *ImageCorruptInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	name := spec.Parameters["name"]
	containerName := spec.Parameters["containerName"]
	corruptImage := spec.Parameters["image"]
	if corruptImage == "" {
		corruptImage = defaultCorruptImage
	}

	key := types.NamespacedName{Name: name, Namespace: namespace}

	deploy := &appsv1.Deployment{}
	if err := i.client.Get(ctx, key, deploy); err != nil {
		return nil, nil, fmt.Errorf("getting Deployment %s: %w", key, err)
	}

	// Find the target container
	containerIdx, err := findContainer(deploy, containerName)
	if err != nil {
		return nil, nil, err
	}

	originalImage := deploy.Spec.Template.Spec.Containers[containerIdx].Image
	targetContainer := deploy.Spec.Template.Spec.Containers[containerIdx].Name
	annotationKey := originalImageAnnotationPrefix + targetContainer

	// Store original image in annotation and patch the container image
	patch := client.MergeFrom(deploy.DeepCopy())
	if deploy.Annotations == nil {
		deploy.Annotations = make(map[string]string)
	}
	deploy.Annotations[annotationKey] = originalImage
	deploy.Annotations[corruptContainerAnnotation] = targetContainer
	deploy.Spec.Template.Spec.Containers[containerIdx].Image = corruptImage

	if err := i.client.Patch(ctx, deploy, patch); err != nil {
		return nil, nil, fmt.Errorf("patching Deployment %s image to corrupt value: %w", key, err)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.ImageCorrupt, key.String(), "image-corrupted",
			map[string]string{
				"namespace":     namespace,
				"deployment":    name,
				"container":     targetContainer,
				"originalImage": originalImage,
				"corruptImage":  corruptImage,
			}),
	}

	cleanup := func(ctx context.Context) error {
		return i.restore(ctx, name, namespace, containerName)
	}

	return cleanup, events, nil
}

func (i *ImageCorruptInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	return i.restore(ctx, spec.Parameters["name"], namespace, spec.Parameters["containerName"])
}

func (i *ImageCorruptInjector) restore(ctx context.Context, name, namespace, containerName string) error {
	key := types.NamespacedName{Name: name, Namespace: namespace}

	deploy := &appsv1.Deployment{}
	if err := i.client.Get(ctx, key, deploy); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting Deployment %s for restore: %w", key, err)
	}

	// Read the stored container name, falling back to the provided containerName
	storedContainer := deploy.Annotations[corruptContainerAnnotation]
	if storedContainer == "" {
		storedContainer = containerName
	}
	if storedContainer == "" {
		return nil
	}

	annotationKey := originalImageAnnotationPrefix + storedContainer
	originalImage, ok := deploy.Annotations[annotationKey]
	if !ok {
		return nil
	}

	containerIdx, err := findContainer(deploy, storedContainer)
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}

	patch := client.MergeFrom(deploy.DeepCopy())
	deploy.Spec.Template.Spec.Containers[containerIdx].Image = originalImage
	delete(deploy.Annotations, annotationKey)
	delete(deploy.Annotations, corruptContainerAnnotation)

	if err := i.client.Patch(ctx, deploy, patch); err != nil {
		return fmt.Errorf("restoring Deployment %s image: %w", key, err)
	}

	return nil
}

// findContainer is defined in crashloop.go and shared with CrashLoopInject.
