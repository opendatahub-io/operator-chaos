# DeploymentScaleZero

**Danger Level:** :material-shield-remove: High

Scales a Deployment to zero replicas, then restores the original replica count on revert.

## Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | Yes | - | Name of the Deployment to scale to zero |
| `ttl` | `duration` | No | `300s` | Auto-cleanup duration |

## How It Works

DeploymentScaleZero patches a Deployment's `spec.replicas` to 0 after storing the original replica count in an annotation (`chaos.operatorchaos.io/original-replicas`). This terminates all pods managed by the Deployment while preserving the Deployment object itself.

**API calls:**
1. `Get` the target Deployment
2. Read current replicas (defaults to 1 if `spec.replicas` is nil)
3. `Patch` the Deployment: set annotation with original count, set replicas to 0
4. On revert: `Get` the Deployment, read the annotation, `Patch` to restore replicas and remove the annotation

**Cleanup:** Restores the original replica count from the annotation. If the Deployment no longer exists, cleanup is a no-op.

**Crash safety:** The original replica count is stored in the Deployment's own annotation, so it survives chaos tool crashes. Manual recovery: read the `chaos.operatorchaos.io/original-replicas` annotation and patch replicas back.

## Disruption Rubric

**Expected behavior on a healthy operator:**
If an OLM-managed operator or higher-level controller owns this Deployment, it should detect the replica count was changed and restore it to the desired value. If no higher-level controller manages the Deployment, the pods stay at zero until manual intervention or revert.

**Contract violation indicators:**
- The owning operator does not detect or correct the scale-to-zero (indicates missing reconciliation of replica count)
- Operator status does not reflect the degraded state (indicates missing health checks or status reporting)
- After revert, pods do not become Ready within `recoveryTimeout` (indicates startup dependency issues)
- Operator enters an unrecoverable state after scaling back up (indicates state loss during zero-replica period)

**Collateral damage risks:**
- **High.** All pods are terminated. Any in-flight requests or connections are dropped immediately
- If the Deployment serves webhooks, API server operations may fail until pods are restored
- Downstream components that depend on this operator will be affected for the entire duration
- On clusters with pod preemption, restoring replicas may evict lower-priority workloads

**Recovery expectations:**
- Recovery time: depends on whether a higher-level controller (OLM, RHODS operator) restores replicas. If no controller intervenes, recovery only happens at revert.
- Reconcile cycles: 1 (if the owning operator restores replicas) or 0 (waits for revert)
- What "recovered" means: Deployment has the original replica count and `Available=True` condition

## Cross-Component Results

| Component | Experiment | Danger | Description |
|-----------|------------|--------|-------------|
| odh-model-controller | odh-model-controller-deployment-scale-zero | high | Scaling odh-model-controller to zero replicas. The RHOAI operator does not restore the replica count, leaving the component degraded until revert. |
| cert-manager | cert-manager-deployment-scale-zero | high | Scaling cert-manager to zero replicas. cert-manager has no higher-level operator watching its replica count, but recovers cleanly on revert. |
| knative-serving | knative-serving-controller-deployment-scale-zero | high | Scaling the knative-serving controller to zero replicas. The Serverless operator detects and restores the replica count. |

<!-- custom-start: notes -->
<!-- custom-end: notes -->
