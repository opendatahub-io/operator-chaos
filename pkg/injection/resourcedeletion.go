package injection

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceDeletionInjector deletes arbitrary namespaced Kubernetes resources
// and stores a backup in a Secret for restoration on revert.
type ResourceDeletionInjector struct {
	client client.Client
}

// NewResourceDeletionInjector creates a new ResourceDeletionInjector.
func NewResourceDeletionInjector(c client.Client) *ResourceDeletionInjector {
	return &ResourceDeletionInjector{client: c}
}

func (r *ResourceDeletionInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateResourceDeletionParams(spec)
}

func resourceDeletionBackupSecretName(kind, name string) string {
	return "chaos-backup-resource-" + strings.ToLower(kind) + "-" + name
}

func (r *ResourceDeletionInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	apiVersion := spec.Parameters["apiVersion"]
	kind := spec.Parameters["kind"]
	name := spec.Parameters["name"]
	targetNamespace := spec.Parameters["namespace"]
	if targetNamespace == "" {
		targetNamespace = namespace
	}

	// Get the resource using unstructured
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)

	key := types.NamespacedName{Name: name, Namespace: targetNamespace}
	if err := r.client.Get(ctx, key, obj); err != nil {
		return nil, nil, fmt.Errorf("getting %s/%s: %w", kind, name, err)
	}

	// Serialize to JSON for backup
	resourceJSON, err := json.Marshal(obj.Object)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing %s/%s: %w", kind, name, err)
	}

	// Store backup in a Secret
	backupSecretName := resourceDeletionBackupSecretName(kind, name)
	chaosLabels := safety.ChaosLabels(string(v1alpha1.ResourceDeletion))

	backupSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupSecretName,
			Namespace: targetNamespace,
			Labels:    chaosLabels,
		},
		Data: map[string][]byte{
			"resource-backup": resourceJSON,
		},
	}

	// Handle backup collision
	var existingSecret corev1.Secret
	if err := r.client.Get(ctx, types.NamespacedName{Name: backupSecretName, Namespace: targetNamespace}, &existingSecret); err == nil {
		if existingSecret.Labels[safety.ManagedByLabel] != chaosLabels[safety.ManagedByLabel] {
			return nil, nil, fmt.Errorf("secret %q already exists in namespace %q and is not chaos-managed; refusing to overwrite", backupSecretName, targetNamespace)
		}
		existingSecret.Labels = chaosLabels
		existingSecret.Data = backupSecret.Data
		if err := r.client.Update(ctx, &existingSecret); err != nil {
			return nil, nil, fmt.Errorf("updating stale backup Secret: %w", err)
		}
	} else if apierrors.IsNotFound(err) {
		if err := r.client.Create(ctx, backupSecret); err != nil {
			return nil, nil, fmt.Errorf("creating backup Secret %s: %w", backupSecretName, err)
		}
	} else {
		return nil, nil, fmt.Errorf("checking for existing backup Secret: %w", err)
	}

	// Delete the resource
	if err := r.client.Delete(ctx, obj); err != nil {
		// Best-effort cleanup of the backup Secret
		cleanupSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupSecretName,
				Namespace: targetNamespace,
			},
		}
		_ = r.client.Delete(ctx, cleanupSecret)
		return nil, nil, fmt.Errorf("deleting %s/%s: %w", kind, name, err)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.ResourceDeletion, key.String(), "deleted",
			map[string]string{
				"namespace":  targetNamespace,
				"kind":       kind,
				"apiVersion": apiVersion,
				"name":       name,
			}),
	}

	cleanup := func(ctx context.Context) error {
		return r.restoreResource(ctx, apiVersion, kind, name, targetNamespace)
	}

	return cleanup, events, nil
}

func (r *ResourceDeletionInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	apiVersion := spec.Parameters["apiVersion"]
	kind := spec.Parameters["kind"]
	name := spec.Parameters["name"]
	targetNamespace := spec.Parameters["namespace"]
	if targetNamespace == "" {
		targetNamespace = namespace
	}
	return r.restoreResource(ctx, apiVersion, kind, name, targetNamespace)
}

func (r *ResourceDeletionInjector) restoreResource(ctx context.Context, apiVersion, kind, name, namespace string) error {
	backupSecretName := resourceDeletionBackupSecretName(kind, name)
	secretKey := types.NamespacedName{Name: backupSecretName, Namespace: namespace}

	var backupSecret corev1.Secret
	if err := r.client.Get(ctx, secretKey, &backupSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting backup Secret %s: %w", secretKey, err)
	}

	// Check if the resource was already recreated (by operator or other means)
	existingObj := &unstructured.Unstructured{}
	existingObj.SetAPIVersion(apiVersion)
	existingObj.SetKind(kind)
	resourceKey := types.NamespacedName{Name: name, Namespace: namespace}

	resourceExists := false
	if err := r.client.Get(ctx, resourceKey, existingObj); err == nil {
		resourceExists = true
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("checking if %s/%s was recreated: %w", kind, name, err)
	}

	// If the resource doesn't exist, restore from backup
	if !resourceExists {
		resourceJSON, ok := backupSecret.Data["resource-backup"]
		if !ok {
			return fmt.Errorf("backup Secret %s missing 'resource-backup' key", secretKey)
		}

		var restoredData map[string]any
		if err := json.Unmarshal(resourceJSON, &restoredData); err != nil {
			return fmt.Errorf("deserializing resource from backup: %w", err)
		}

		restored := &unstructured.Unstructured{Object: restoredData}

		// Clear server-managed fields so the resource can be recreated cleanly
		restored.SetUID("")
		restored.SetResourceVersion("")
		restored.SetCreationTimestamp(metav1.Time{})
		restored.SetManagedFields(nil)
		restored.SetDeletionTimestamp(nil)
		restored.SetDeletionGracePeriodSeconds(nil)
		restored.SetGeneration(0)
		delete(restored.Object, "status")
		restored.SetFinalizers(nil)

		if err := r.client.Create(ctx, restored); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("restoring %s/%s/%s: %w", namespace, kind, name, err)
			}
		}
	}

	// Always clean up the backup Secret
	if err := r.client.Delete(ctx, &backupSecret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting backup Secret %s: %w", secretKey, err)
	}

	return nil
}
