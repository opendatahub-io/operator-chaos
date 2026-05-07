package injection

import (
	"context"
	"encoding/json"
	"fmt"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LeaseElectionInjector struct {
	client client.Client
}

func NewLeaseElectionInjector(c client.Client) *LeaseElectionInjector {
	return &LeaseElectionInjector{client: c}
}

func (l *LeaseElectionInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validateLeaseElectionParams(spec)
}

func leaseBackupConfigMapName(leaseName string) string {
	return "chaos-backup-lease-" + leaseName
}

func (l *LeaseElectionInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	leaseName := spec.Parameters["name"]
	key := types.NamespacedName{Name: leaseName, Namespace: namespace}

	lease := &coordinationv1.Lease{}
	if err := l.client.Get(ctx, key, lease); err != nil {
		return nil, nil, fmt.Errorf("getting Lease %s: %w", key, err)
	}

	// Serialize the full Lease spec to JSON for backup
	leaseJSON, err := json.Marshal(lease)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing Lease %s: %w", key, err)
	}

	// Store backup in a ConfigMap
	backupCMName := leaseBackupConfigMapName(leaseName)
	chaosLabels := safety.ChaosLabels(string(v1alpha1.LeaderElectionDisrupt))

	backupCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupCMName,
			Namespace: namespace,
			Labels:    chaosLabels,
		},
		Data: map[string]string{
			"lease-backup": string(leaseJSON),
		},
	}

	var existingCM corev1.ConfigMap
	if err := l.client.Get(ctx, types.NamespacedName{Name: backupCMName, Namespace: namespace}, &existingCM); err == nil {
		if existingCM.Labels[safety.ManagedByLabel] != chaosLabels[safety.ManagedByLabel] {
			return nil, nil, fmt.Errorf("configMap %q already exists in namespace %q and is not chaos-managed; refusing to overwrite", backupCMName, namespace)
		}
		existingCM.Labels = chaosLabels
		existingCM.Data = backupCM.Data
		if err := l.client.Update(ctx, &existingCM); err != nil {
			return nil, nil, fmt.Errorf("updating stale backup ConfigMap: %w", err)
		}
	} else if apierrors.IsNotFound(err) {
		if err := l.client.Create(ctx, backupCM); err != nil {
			return nil, nil, fmt.Errorf("creating backup ConfigMap %s: %w", backupCMName, err)
		}
	} else {
		return nil, nil, fmt.Errorf("checking for existing backup ConfigMap: %w", err)
	}

	// Delete the Lease to force re-election
	if err := l.client.Delete(ctx, lease); err != nil {
		// Best-effort cleanup of the backup ConfigMap
		cleanupCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupCMName,
				Namespace: namespace,
			},
		}
		_ = l.client.Delete(ctx, cleanupCM)
		return nil, nil, fmt.Errorf("deleting Lease %s: %w", key, err)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.LeaderElectionDisrupt, key.String(), "deleted",
			map[string]string{
				"namespace": namespace,
				"lease":     leaseName,
			}),
	}

	cleanup := func(ctx context.Context) error {
		return l.restoreLease(ctx, leaseName, namespace)
	}

	return cleanup, events, nil
}

func (l *LeaseElectionInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	return l.restoreLease(ctx, spec.Parameters["name"], namespace)
}

func (l *LeaseElectionInjector) restoreLease(ctx context.Context, leaseName, namespace string) error {
	backupCMName := leaseBackupConfigMapName(leaseName)
	cmKey := types.NamespacedName{Name: backupCMName, Namespace: namespace}

	var backupCM corev1.ConfigMap
	if err := l.client.Get(ctx, cmKey, &backupCM); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting backup ConfigMap %s: %w", cmKey, err)
	}

	// Check if the operator already recreated the Lease
	leaseKey := types.NamespacedName{Name: leaseName, Namespace: namespace}
	var existingLease coordinationv1.Lease
	leaseExists := false
	if err := l.client.Get(ctx, leaseKey, &existingLease); err == nil {
		leaseExists = true
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("checking if Lease %s was recreated: %w", leaseKey, err)
	}

	// If the operator hasn't recreated the Lease, restore from backup
	if !leaseExists {
		leaseJSON, ok := backupCM.Data["lease-backup"]
		if !ok {
			return fmt.Errorf("backup ConfigMap %s missing 'lease-backup' key", cmKey)
		}

		var restoredLease coordinationv1.Lease
		if err := json.Unmarshal([]byte(leaseJSON), &restoredLease); err != nil {
			return fmt.Errorf("deserializing Lease from backup: %w", err)
		}

		// Clear server-managed fields so the Lease can be recreated cleanly
		restoredLease.UID = ""
		restoredLease.ResourceVersion = ""
		restoredLease.CreationTimestamp = metav1.Time{}
		restoredLease.ManagedFields = nil

		if err := l.client.Create(ctx, &restoredLease); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("restoring Lease %s/%s: %w", namespace, leaseName, err)
			}
		}
	}

	// Clean up the backup ConfigMap
	if err := l.client.Delete(ctx, &backupCM); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting backup ConfigMap %s: %w", cmKey, err)
	}

	return nil
}
