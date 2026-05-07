package injection

import (
	"context"
	"fmt"
	"sort"
	"strings"

	v1alpha1 "github.com/opendatahub-io/operator-chaos/api/v1alpha1"
	"github.com/opendatahub-io/operator-chaos/pkg/safety"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PDBBlockInjector creates a PodDisruptionBudget with maxUnavailable=0
// to block voluntary evictions.
type PDBBlockInjector struct {
	client client.Client
}

// NewPDBBlockInjector creates a new PDBBlockInjector.
func NewPDBBlockInjector(c client.Client) *PDBBlockInjector {
	return &PDBBlockInjector{client: c}
}

func (p *PDBBlockInjector) Validate(spec v1alpha1.InjectionSpec, blast v1alpha1.BlastRadiusSpec) error {
	return validatePDBBlockParams(spec)
}

// parseLabelSelector parses a comma-separated key=value label selector string
// into a map. Each part must contain exactly one "=".
func parseLabelSelector(selector string) (map[string]string, error) {
	result := make(map[string]string)
	parts := strings.Split(selector, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("invalid label selector part %q: expected key=value", part)
		}
		result[kv[0]] = kv[1]
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("label selector produced no labels")
	}
	return result, nil
}

// generatePDBName generates a deterministic PDB name from the first label value
// (sorted by key) if no name is provided.
func generatePDBName(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		return "chaos-pdb-" + labels[keys[0]]
	}
	return "chaos-pdb-unknown"
}

func (p *PDBBlockInjector) Inject(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	selectorStr := spec.Parameters["labelSelector"]
	labelMap, err := parseLabelSelector(selectorStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing labelSelector: %w", err)
	}

	pdbName := spec.Parameters["name"]
	if pdbName == "" {
		pdbName = generatePDBName(labelMap)
	}

	chaosLabels := safety.ChaosLabels(string(v1alpha1.PDBBlock))
	maxUnavailable := intstr.FromInt32(0)

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pdbName,
			Namespace: namespace,
			Labels:    chaosLabels,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &maxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: labelMap,
			},
		},
	}

	// Check for existing PDB
	var existing policyv1.PodDisruptionBudget
	key := types.NamespacedName{Name: pdbName, Namespace: namespace}
	if err := p.client.Get(ctx, key, &existing); err == nil {
		// PDB already exists
		if existing.Labels[safety.ManagedByLabel] != chaosLabels[safety.ManagedByLabel] {
			return nil, nil, fmt.Errorf("pdb %q already exists in namespace %q and is not chaos-managed; refusing to overwrite", pdbName, namespace)
		}
		// Update existing chaos-managed PDB
		existing.Labels = chaosLabels
		existing.Spec = pdb.Spec
		if err := p.client.Update(ctx, &existing); err != nil {
			return nil, nil, fmt.Errorf("updating existing chaos-managed PDB %s: %w", pdbName, err)
		}
	} else if apierrors.IsNotFound(err) {
		if err := p.client.Create(ctx, pdb); err != nil {
			return nil, nil, fmt.Errorf("creating PDB %s: %w", pdbName, err)
		}
	} else {
		return nil, nil, fmt.Errorf("checking for existing PDB: %w", err)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.PDBBlock, key.String(), "created",
			map[string]string{
				"namespace":     namespace,
				"pdbName":       pdbName,
				"labelSelector": selectorStr,
			}),
	}

	cleanup := func(ctx context.Context) error {
		return p.deletePDB(ctx, pdbName, namespace)
	}

	return cleanup, events, nil
}

func (p *PDBBlockInjector) Revert(ctx context.Context, spec v1alpha1.InjectionSpec, namespace string) error {
	selectorStr := spec.Parameters["labelSelector"]
	pdbName := spec.Parameters["name"]
	if pdbName == "" {
		labelMap, err := parseLabelSelector(selectorStr)
		if err != nil {
			return fmt.Errorf("parsing labelSelector for revert: %w", err)
		}
		pdbName = generatePDBName(labelMap)
	}
	return p.deletePDB(ctx, pdbName, namespace)
}

func (p *PDBBlockInjector) deletePDB(ctx context.Context, pdbName, namespace string) error {
	key := types.NamespacedName{Name: pdbName, Namespace: namespace}

	var pdb policyv1.PodDisruptionBudget
	if err := p.client.Get(ctx, key, &pdb); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting PDB %s for cleanup: %w", key, err)
	}

	// Only delete if chaos-managed
	chaosLabels := safety.ChaosLabels(string(v1alpha1.PDBBlock))
	if pdb.Labels[safety.ManagedByLabel] != chaosLabels[safety.ManagedByLabel] {
		return fmt.Errorf("pdb %q is not chaos-managed; refusing to delete", pdbName)
	}

	if err := p.client.Delete(ctx, &pdb); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting PDB %s: %w", key, err)
	}

	return nil
}
