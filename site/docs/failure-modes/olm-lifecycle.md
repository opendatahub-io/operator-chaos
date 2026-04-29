# OLM Lifecycle Disruption

Corrupt OLM (Operator Lifecycle Manager) resources to test operator resilience to lifecycle management failures. These experiments target the Subscription, ClusterServiceVersion, and InstallPlan objects that OLM uses to install and upgrade operators.

**Tier:** 3 (CRDMutation)  
**Danger:** Medium-High. Mutations affect OLM's ability to manage operator upgrades but should not disrupt the running operator deployment.

## How It Works

OLM lifecycle experiments use the [CRDMutation](crd-mutation.md) injection type to modify OLM resources via merge patch. The chaos framework stores the original value in a rollback annotation and restores it after the observation period.

All three scenarios exploit the fact that OLM resources control the upgrade lifecycle, not the runtime lifecycle. Corrupting a Subscription or CSV should not cause the operator pods to restart or become unavailable.

## Scenarios

### Subscription Channel Corrupt

Mutates `spec.channel` to a non-existent channel name.

```yaml
injection:
  type: CRDMutation
  parameters:
    apiVersion: "operators.coreos.com/v1alpha1"
    kind: "Subscription"
    name: "opendatahub-operator"
    path: "spec.channel"
    value: "nonexistent-channel"
```

**What to expect:**

- OLM marks the Subscription as unhealthy (no matching channel in catalog)
- Operator deployment remains running and Available
- No new InstallPlans are created (channel resolution fails)
- After rollback, OLM resumes normal channel tracking

### Subscription Approval Flip

Changes `spec.installPlanApproval` from `Automatic` to `Manual`.

```yaml
injection:
  type: CRDMutation
  parameters:
    apiVersion: "operators.coreos.com/v1alpha1"
    kind: "Subscription"
    name: "opendatahub-operator"
    path: "spec.installPlanApproval"
    value: "Manual"
```

**What to expect:**

- If an upgrade is pending, the InstallPlan stays in `RequiresApproval` state
- Currently running operator is unaffected
- After rollback, approval mode returns to Automatic and any pending InstallPlan is auto-approved

### CSV Owned CRD Corrupt

Empties the CSV's `spec.customresourcedefinitions.owned` list. Requires `dangerLevel: high` because the value is a JSON array.

```yaml
injection:
  type: CRDMutation
  parameters:
    apiVersion: "operators.coreos.com/v1alpha1"
    kind: "ClusterServiceVersion"
    name: "opendatahub-operator.v2.22.0"
    path: "spec.customresourcedefinitions.owned"
    value: "[]"
  dangerLevel: high
```

**What to expect:**

- OLM detects the CSV spec is inconsistent with the installed CRDs
- CSV may transition to a degraded state
- Operator deployment remains running (OLM does not delete operator pods for metadata corruption)
- After rollback, OLM re-validates the CSV and returns to Succeeded phase

!!! warning "CSV name includes version"
    The CSV name includes the operator version (e.g., `opendatahub-operator.v2.22.0` or `rhods-operator.3.3.2`). You must update the `name` parameter to match the currently installed CSV. Find it with:
    ```bash
    oc get csv -n openshift-operators | grep opendatahub
    # or for RHOAI:
    oc get csv -n redhat-ods-operator | grep rhods
    ```

## When to Use

- **Upgrade pipeline validation**: Verify that channel corruption or approval mode changes don't crash the running operator
- **OLM dependency testing**: Confirm the operator handles OLM state transitions gracefully
- **Disaster recovery**: Test recovery procedures when OLM resources are accidentally modified

## Namespace Considerations

OLM resources live in the operator's install namespace, not the applications namespace:

| Distribution | Subscription Namespace | CSV Namespace |
|-------------|----------------------|---------------|
| ODH | `openshift-operators` | `openshift-operators` |
| RHOAI | `redhat-ods-operator` | `redhat-ods-operator` |

The experiments set `allowedNamespaces` accordingly. The `--namespace` CLI flag overrides the resolved namespace for CRDMutation. If you need to target a specific namespace, set a single entry in `allowedNamespaces` in the experiment spec.

## Available Experiments

### ODH (upstream)

| Experiment | File |
|-----------|------|
| Subscription Channel Corrupt | `experiments/opendatahub-operator/olm-subscription-channel-corrupt.yaml` |
| Subscription Approval Flip | `experiments/opendatahub-operator/olm-subscription-approval-flip.yaml` |
| CSV Owned CRD Corrupt | `experiments/opendatahub-operator/olm-csv-owned-crd-corrupt.yaml` |

### RHOAI (downstream)

| Experiment | File |
|-----------|------|
| Subscription Channel Corrupt | `experiments/rhoai/opendatahub-operator/olm-subscription-channel-corrupt.yaml` |
| Subscription Approval Flip | `experiments/rhoai/opendatahub-operator/olm-subscription-approval-flip.yaml` |
| CSV Owned CRD Corrupt | `experiments/rhoai/opendatahub-operator/olm-csv-owned-crd-corrupt.yaml` |
