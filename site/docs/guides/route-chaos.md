# Route Chaos Experiments

OpenShift Routes are critical networking resources that control external access to RHOAI components. Route chaos experiments verify that operators correctly reconcile Route resources when they drift from their expected state.

All Route chaos scenarios use the existing **CRDMutation** injection type against `route.openshift.io/v1` resources. No new injection type is needed. Route mutations require `dangerLevel: high` because `route.openshift.io/` is treated as an infrastructure type in the validation layer.

## Why Route Chaos Matters

Routes are the ingress layer for RHOAI components. A misconfigured Route can:

- Make the dashboard completely unreachable (host collision, TLS corruption)
- Break model inference endpoints (KServe Route mutation)
- Disrupt model registry API access (backend service mismatch)
- Cause silent failures where the Route exists but serves errors (503 backend not found)

Unlike pod-level faults, Route mutations affect the networking layer and test whether operators monitor and reconcile infrastructure resources, not just their own deployments.

## Components with Routes

| Component | Route Name | Purpose |
|-----------|-----------|---------|
| Dashboard | `odh-dashboard` / `rhods-dashboard` | Dashboard web UI |
| Model Registry | `model-registry-operator-rest` | REST API endpoint |
| KServe | Per-InferenceService (varies) | Model inference endpoints |

## Experiment Scenarios

### 1. Route Host Collision

Mutates `spec.host` to a cluster-like domain that doesn't match the actual cluster's ingress configuration.

```yaml
injection:
  type: CRDMutation
  dangerLevel: high
  parameters:
    apiVersion: "route.openshift.io/v1"
    kind: "Route"
    name: "rhods-dashboard"
    path: "spec.host"
    value: "chaos-collision.apps.cluster.invalid"
```

**What happens:** The OpenShift router re-evaluates the Route. Since the host doesn't match any configured ingress domain, the Route becomes unreachable. The operator should detect the drift and restore the original host.

**Files:** `experiments/dashboard/route-host-collision.yaml`, `experiments/model-registry/route-host-collision.yaml`

### 2. Route TLS Cert Mutation

Changes the TLS termination mode from `edge` or `reencrypt` to `passthrough`.

```yaml
injection:
  type: CRDMutation
  dangerLevel: high
  parameters:
    apiVersion: "route.openshift.io/v1"
    kind: "Route"
    name: "rhods-dashboard"
    path: "spec.tls.termination"
    value: "passthrough"
```

**What happens:** The router stops terminating TLS and forwards encrypted traffic directly to the backend pod. Since RHOAI pods typically don't serve TLS themselves, HTTPS access breaks. The operator should reconcile the TLS settings back to their correct mode.

**Files:** `experiments/dashboard/route-tls-mutation.yaml`, `experiments/model-registry/route-tls-mutation.yaml`

### 3. Route Host Deletion

Deletes the Route's `spec.host` field via null merge patch, which indirectly causes the router to de-admit the Route.

```yaml
injection:
  type: CRDMutation
  dangerLevel: high
  parameters:
    apiVersion: "route.openshift.io/v1"
    kind: "Route"
    name: "odh-dashboard"
    path: "spec.host"
    value: "null"
```

**What happens:** The Route loses its host assignment. Without a host, the router de-admits the Route and clears its admission status. The operator should detect the missing host and restore the Route configuration. This scenario indirectly tests status clearing behavior: removing the host causes the router to clear the Route's `status.ingress` admission entries.

**Files:** `experiments/dashboard/route-host-deletion.yaml`

### 4. Router Shard Mismatch

Sets `spec.host` to a completely non-routable local domain that no IngressController will claim.

```yaml
injection:
  type: CRDMutation
  dangerLevel: high
  parameters:
    apiVersion: "route.openshift.io/v1"
    kind: "Route"
    name: "odh-dashboard"
    path: "spec.host"
    value: "dashboard.nonexistent-shard.local"
```

**What happens:** Unlike host-collision (which uses a cluster-like `.apps.cluster.invalid` domain), this targets a completely non-routable `.local` domain. No IngressController will claim the Route, making it unserviceable. The operator should detect the orphaned Route and reconcile the host back to its original value.

**Files:** `experiments/dashboard/route-shard-mismatch.yaml`

### 5. Route Backend Disruption (Admission Denial)

Changes `spec.to.name` to point at a non-existent backend Service.

```yaml
injection:
  type: CRDMutation
  dangerLevel: high
  parameters:
    apiVersion: "route.openshift.io/v1"
    kind: "Route"
    name: "rhods-dashboard"
    path: "spec.to.name"
    value: "chaos-nonexistent-service"
```

**What happens:** The Route remains admitted by the router, but all requests return HTTP 503 because the backend Service doesn't exist. The operator should detect the broken backend reference and restore it.

**Files:** `experiments/dashboard/route-backend-disruption.yaml`, `experiments/model-registry/route-backend-disruption.yaml`

## Running Route Experiments

### Prerequisites

- OpenShift cluster with RHOAI installed
- `cluster-admin` RBAC
- Routes must exist for the target components

### Verify Routes Exist

```bash
oc get routes -n redhat-ods-applications
oc get routes -n odh-model-registries
```

### Run a Single Experiment

```bash
# ODH
operator-chaos run experiments/dashboard/route-host-collision.yaml \
  --knowledge knowledge/dashboard.yaml \
  --namespace opendatahub -v

# RHOAI
operator-chaos run experiments/rhoai/dashboard/route-host-collision.yaml \
  --knowledge knowledge/rhoai/dashboard.yaml \
  --namespace redhat-ods-applications -v
```

### Run All Route Experiments for a Component

```bash
operator-chaos suite experiments/dashboard/ \
  --knowledge knowledge/dashboard.yaml \
  --namespace opendatahub \
  --report-dir /tmp/route-chaos/
```

## KServe Route Experiments

KServe creates Routes dynamically per InferenceService. The Route name follows the pattern `<isvc-name>-predictor` in the model namespace. Before running KServe Route experiments, update the `name` parameter:

```bash
# Find the actual Route name
oc get routes -n <model-namespace> | grep predictor

# Edit the experiment file or use --set-param (if supported)
```

Template files are provided at `experiments/kserve/route-host-collision.yaml` and `experiments/kserve/route-tls-mutation.yaml`. Note that KServe Routes are not included in the kserve knowledge model because they are created dynamically per InferenceService rather than statically by the operator.

## Expected Verdicts

| Scenario | Resilient | Vulnerable |
|----------|-----------|------------|
| Host collision | Operator restores original host | Route stays misconfigured |
| TLS mutation | Operator restores TLS termination mode | HTTPS access broken |
| Host deletion | Operator restores host field | Route remains un-admitted |
| Shard mismatch | Operator restores host to correct domain | Route orphaned from all shards |
| Backend disruption | Operator restores backend service name | 503 errors persist |

## Safety Notes

All Route experiments require `dangerLevel: high` and `allowDangerous: true` because `route.openshift.io/` is classified as an infrastructure type (alongside core Kubernetes types like `apps/`, `rbac.authorization.k8s.io/`, etc.) in the CRDMutation validation. This ensures Route mutations are never accidentally run without explicit acknowledgment of the risk.

The TTL-based cleanup (default 300s) ensures the original Route configuration is restored even if the operator doesn't reconcile. The CRDMutation injector stores rollback data in annotations with SHA-256 integrity verification, making cleanup crash-safe.
