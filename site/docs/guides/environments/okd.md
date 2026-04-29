# OKD

[OKD](https://www.okd.io/) is the community distribution of Kubernetes that powers OpenShift. It provides the same API surface as OCP (Routes, OLM, SCCs, admission webhooks) without requiring a Red Hat subscription, making it ideal for contributors and community testing.

## Supported Tiers

| Tier | Supported | Notes |
|------|-----------|-------|
| 1 (PodKill) | Yes | Full support |
| 2 (ConfigDrift, NetworkPartition) | Yes | OVN-Kubernetes enforces NetworkPolicies |
| 3 (CRDMutation, FinalizerBlock, LabelStomping) | Yes | Routes, Gateway API, owner references |
| 4 (WebhookDisrupt, RBACRevoke, WebhookLatency) | Yes | Full admission webhook and RBAC testing |
| 5 (NamespaceDeletion, QuotaExhaustion) | Yes | Same behavior as OCP |
| 6 (Multi-fault, upgrades) | Yes | OLM upgrades work the same as OCP |

OKD supports the same tiers as OCP. The main difference is operator catalog sources: ODH uses `community-operators` on OKD, while RHOAI is not available (it requires an OCP subscription).

## Cluster Setup

### OKD on cloud infrastructure

Follow the [OKD installation docs](https://docs.okd.io/latest/welcome/index.html). OKD supports the same installation methods as OCP: IPI (installer-provisioned), UPI (user-provisioned), and single-node.

```bash
# Download the OKD installer from https://github.com/okd-project/okd/releases/latest
# Asset names include the version, e.g.:
curl -L https://github.com/okd-project/okd/releases/download/<VERSION>/openshift-install-linux-<VERSION>.tar.gz | tar xz

# Create install config
./openshift-install create install-config --dir=okd-cluster

# Deploy
./openshift-install create cluster --dir=okd-cluster
```

### OKD on local machine (SNO)

Single Node OKD for development:

```bash
# Requires 8 CPU, 32GB RAM minimum
./openshift-install create single-node-ignition-config --dir=okd-sno
```

## Operator Installation

Install ODH on OKD the same way as on OCP, using the community catalog:

```bash
cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: opendatahub-operator
  namespace: openshift-operators
spec:
  channel: fast
  name: opendatahub-operator
  source: community-operators
  sourceNamespace: openshift-marketplace
EOF

# Wait for operator
oc wait --for=condition=Available deployment/opendatahub-operator \
  -n openshift-operators --timeout=120s

# Create DSCInitialization and DataScienceCluster
oc apply -f - <<EOF
apiVersion: dscinitialization.opendatahub.io/v1
kind: DSCInitialization
metadata:
  name: default-dsci
spec:
  applicationsNamespace: opendatahub
EOF

oc apply -f - <<EOF
apiVersion: datasciencecluster.opendatahub.io/v1
kind: DataScienceCluster
metadata:
  name: default-dsc
spec:
  components:
    dashboard:
      managementState: Managed
    kserve:
      managementState: Managed
    workbenches:
      managementState: Managed
    modelregistry:
      managementState: Managed
EOF
```

## Running Experiments

The workflow is identical to OCP. Use the ODH knowledge models and experiments (not the RHOAI-specific ones under `experiments/rhoai/`):

```bash
# Install
go install github.com/opendatahub-io/operator-chaos/cmd/operator-chaos@latest

# Preflight
operator-chaos preflight --knowledge knowledge/odh-model-controller.yaml

# Run experiments progressively
operator-chaos suite experiments/odh-model-controller/ \
  --knowledge knowledge/odh-model-controller.yaml \
  --max-tier 2 --cooldown 30s \
  --report-dir /tmp/chaos-results/
```

## Differences from OCP

| Feature | OCP | OKD |
|---------|-----|-----|
| RHOAI availability | Yes (redhat-operators catalog) | No (ODH only) |
| Operator catalog | redhat-operators + community-operators | community-operators only |
| Support | Red Hat subscription | Community |
| Release cadence | Quarterly | Follows OCP releases |
| Default namespace | `redhat-ods-applications` (RHOAI) or `opendatahub` (ODH) | `opendatahub` |

For chaos testing purposes, the platform APIs are identical. Experiments written for OCP work on OKD without modification (when using ODH, not RHOAI).

## Next Steps

- [OCP guide](ocp.md) if you have an OCP subscription and need RHOAI testing
- [CLI Quickstart](../../modes/cli.md) for the full experiment workflow
- [E2E Testing](../e2e-testing.md) for comprehensive test strategies
