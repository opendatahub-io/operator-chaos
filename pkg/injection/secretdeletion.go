package injection

import (
	"context"
	"encoding/json"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretDeletionInjector struct {
	client client.Client
}

func NewSecretDeletionInjector(c client.Client) *SecretDeletionInjector {
	return &SecretDeletionInjector{client: c}
}

func (s *SecretDeletionInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateSecretDeletionParams(spec)
}

func backupSecretName(secretName string) string {
	return "chaos-backup-secret-" + secretName
}

func (s *SecretDeletionInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	secretName := spec.Parameters["name"]
	targetNamespace := spec.Parameters["namespace"]
	if targetNamespace == "" {
		targetNamespace = namespace
	}

	key := types.NamespacedName{Name: secretName, Namespace: targetNamespace}

	secret := &corev1.Secret{}
	if err := s.client.Get(ctx, key, secret); err != nil {
		return nil, nil, fmt.Errorf("getting Secret %s: %w", key, err)
	}

	// Serialize the full Secret to JSON for backup
	secretJSON, err := json.Marshal(secret)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing Secret %s: %w", key, err)
	}

	// Store backup in a Secret
	bkpName := backupSecretName(secretName)
	chaosLabels := safety.ChaosLabels(string(v1alpha1.SecretDeletion))

	backupSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bkpName,
			Namespace: targetNamespace,
			Labels:    chaosLabels,
		},
		Data: map[string][]byte{
			"secret-backup": secretJSON,
		},
	}

	var existingSecret corev1.Secret
	if err := s.client.Get(ctx, types.NamespacedName{Name: bkpName, Namespace: targetNamespace}, &existingSecret); err == nil {
		if existingSecret.Labels[safety.ManagedByLabel] != chaosLabels[safety.ManagedByLabel] {
			return nil, nil, fmt.Errorf("backup Secret %q already exists in namespace %q and is not chaos-managed; refusing to overwrite", bkpName, targetNamespace)
		}
		existingSecret.Labels = chaosLabels
		existingSecret.Data = backupSecret.Data
		if err := s.client.Update(ctx, &existingSecret); err != nil {
			return nil, nil, fmt.Errorf("updating stale backup Secret: %w", err)
		}
	} else if apierrors.IsNotFound(err) {
		if err := s.client.Create(ctx, backupSecret); err != nil {
			return nil, nil, fmt.Errorf("creating backup Secret %s: %w", bkpName, err)
		}
	} else {
		return nil, nil, fmt.Errorf("checking for existing backup Secret: %w", err)
	}

	// Delete the Secret
	if err := s.client.Delete(ctx, secret); err != nil {
		// Best-effort cleanup of the backup Secret
		cleanupSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bkpName,
				Namespace: targetNamespace,
			},
		}
		_ = s.client.Delete(ctx, cleanupSecret)
		return nil, nil, fmt.Errorf("deleting Secret %s: %w", key, err)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.SecretDeletion, key.String(), "deleted",
			map[string]string{
				"namespace": targetNamespace,
				"secret":    secretName,
			}),
	}

	cleanup := func(ctx context.Context) error {
		return s.restoreSecret(ctx, secretName, targetNamespace)
	}

	return cleanup, events, nil
}

func (s *SecretDeletionInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	secretName := spec.Parameters["name"]
	targetNamespace := spec.Parameters["namespace"]
	if targetNamespace == "" {
		targetNamespace = namespace
	}
	return s.restoreSecret(ctx, secretName, targetNamespace)
}

func (s *SecretDeletionInjector) restoreSecret(ctx context.Context, secretName, namespace string) error {
	bkpName := backupSecretName(secretName)
	bkpKey := types.NamespacedName{Name: bkpName, Namespace: namespace}

	var backupSecret corev1.Secret
	if err := s.client.Get(ctx, bkpKey, &backupSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting backup Secret %s: %w", bkpKey, err)
	}

	secretJSON, ok := backupSecret.Data["secret-backup"]
	if !ok {
		return fmt.Errorf("backup Secret %s missing 'secret-backup' key", bkpKey)
	}

	var restoredSecret corev1.Secret
	if err := json.Unmarshal(secretJSON, &restoredSecret); err != nil {
		return fmt.Errorf("deserializing Secret from backup: %w", err)
	}

	// Clear server-managed fields so the Secret can be recreated cleanly
	restoredSecret.UID = ""
	restoredSecret.ResourceVersion = ""
	restoredSecret.CreationTimestamp = metav1.Time{}
	restoredSecret.ManagedFields = nil

	if err := s.client.Create(ctx, &restoredSecret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Secret was already recreated (by operator or prior revert), proceed to clean up
		} else {
			return fmt.Errorf("restoring Secret %s/%s: %w", namespace, secretName, err)
		}
	}

	// Clean up the backup Secret
	if err := s.client.Delete(ctx, &backupSecret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting backup Secret %s: %w", bkpKey, err)
	}

	return nil
}
