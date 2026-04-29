# OpenShift (OCP / ROSA)

OpenShift is the primary target platform for operator-chaos. It supports all injection types and fidelity tiers, including OLM lifecycle testing, Route chaos, webhook disruption, and namespace deletion.

## Supported Tiers

| Tier | Supported | Notes |
|------|-----------|-------|
| 1 (PodKill) | Yes | Full support |
| 2 (ConfigDrift, NetworkPartition) | Yes | OVN-Kubernetes enforces NetworkPolicies |
| 3 (CRDMutation, FinalizerBlock, LabelStomping) | Yes | Route mutations, Gateway API, owner reference testing |
| 4 (WebhookDisrupt, RBACRevoke, WebhookLatency) | Yes | Full admission webhook and RBAC testing |
| 5 (NamespaceDeletion, QuotaExhaustion) | Yes | Requires `allowDangerous: true` in experiment spec |
| 6 (Multi-fault, upgrades) | Yes | Combined faults during OLM upgrades |

## Cluster Options

### ROSA (Red Hat OpenShift on AWS)

Fastest way to get a managed OCP cluster:

```bash
# Create a ROSA HyperShift cluster
rosa create cluster --cluster-name chaos-test \
  --sts --mode auto \
  --hosted-cp \
  --region us-east-1 \
  --compute-machine-type m5.xlarge \
  --replicas 2
```

!!! warning "Minimum cluster size"
    RHOAI components (especially the dashboard with 5 containers per pod) require significant CPU. A 2x m5.xlarge cluster (7 CPU total) is the minimum for basic testing. For accurate Tier 3+ results, use 3+ nodes or m5.2xlarge instances. The `preflight` command warns when operator deployments exceed 80% of allocatable capacity.

### Self-managed OCP

Follow the [OpenShift installation docs](https://docs.openshift.com/container-platform/latest/installing/index.html) for your infrastructure. Any OCP 4.14+ cluster works.

### CRC (CodeReady Containers)

For local development on a single machine:

```bash
crc setup
crc start --cpus 6 --memory 16384
eval $(crc oc-env)
```

CRC provides a single-node OCP cluster. It supports all tiers but performance is limited. Not recommended for Tier 5 (NamespaceDeletion) as recovery can be slow on a single node.

## Operator Installation

### ODH (upstream)

```bash
# Install ODH operator via OLM
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

# Wait for operator to be ready
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

### RHOAI (downstream)

RHOAI is installed via the OpenShift console or CLI from the `redhat-operators` catalog. The namespace is `redhat-ods-applications` instead of `opendatahub`.

```bash
# Verify installation
oc get pods -n redhat-ods-applications
```

When running experiments, use the RHOAI-specific knowledge models and experiments:

```bash
operator-chaos preflight --knowledge knowledge/dashboard.yaml \
  --namespace redhat-ods-applications
```

## Running Experiments

### Install operator-chaos

```bash
go install github.com/opendatahub-io/operator-chaos/cmd/operator-chaos@latest
```

### Preflight check

Always run preflight before live experiments to verify the knowledge model matches the cluster:

```bash
operator-chaos preflight --knowledge knowledge/odh-model-controller.yaml
```

### Progressive tier execution

Start with low tiers and work up:

```bash
# Tier 1: safe pod kills
operator-chaos suite experiments/odh-model-controller/ \
  --knowledge knowledge/odh-model-controller.yaml \
  --max-tier 1 --report-dir /tmp/chaos-t1/

# Tier 2: config and network faults
operator-chaos suite experiments/odh-model-controller/ \
  --knowledge knowledge/odh-model-controller.yaml \
  --max-tier 2 --cooldown 30s --report-dir /tmp/chaos-t2/

# Tier 3-4: CRD mutations, webhooks, RBAC
operator-chaos suite experiments/odh-model-controller/ \
  --knowledge knowledge/odh-model-controller.yaml \
  --max-tier 4 --cooldown 60s --report-dir /tmp/chaos-t4/

# Tier 5: namespace deletion (dangerous, requires allowDangerous in experiment)
operator-chaos run experiments/odh-model-controller/namespace-deletion.yaml \
  --knowledge knowledge/odh-model-controller.yaml -v
```

!!! tip "Use cooldown for higher tiers"
    Disruptive experiments (Tier 3+) can leave the cluster temporarily degraded. Use `--cooldown 30s` or higher to give the cluster time to stabilize between experiments.

### Route and Gateway API experiments

OCP supports both OpenShift Routes and Gateway API. Route experiments target resources in the `openshift-ingress` namespace and require `allowDangerous: true`:

```bash
# Gateway API experiments (HTTPRoute in applications namespace)
operator-chaos run experiments/dashboard/gateway-backend-disruption.yaml \
  --knowledge knowledge/dashboard.yaml -v

# Route experiments (Route in openshift-ingress, requires allowDangerous)
operator-chaos run experiments/dashboard/route-gateway-backend-disruption.yaml \
  --knowledge knowledge/dashboard.yaml -v
```

### Cleanup

After testing, remove any leftover chaos artifacts:

```bash
operator-chaos clean --namespace redhat-ods-applications
```

## OCP-specific Considerations

### Resource constraints

The `preflight` command checks resource capacity automatically. On small clusters, you may need to:

- Scale down dashboard replicas: `oc scale deployment rhods-dashboard --replicas=1 -n redhat-ods-applications`
- Reduce CPU requests on resource-heavy pods

### Webhook experiments

OCP manages admission webhooks for its own components. WebhookDisrupt experiments (Tier 4) temporarily modify ValidatingWebhookConfigurations or MutatingWebhookConfigurations. The operator should restore them, but always verify with `operator-chaos clean` after testing.

### Namespace deletion

NamespaceDeletion (Tier 5) deletes the target namespace and waits for the operator to recreate it. On OCP/RHOAI, this is an extreme test: the operator currently does not auto-recover from namespace deletion. Use this only for pre-release validation on non-production clusters.

## Next Steps

- [E2E Testing](../e2e-testing.md) for comprehensive test strategies
- [CI Integration](../ci-integration.md) for automated pipeline setup
- [Route Chaos](../route-chaos.md) for OCP-specific Route testing patterns
