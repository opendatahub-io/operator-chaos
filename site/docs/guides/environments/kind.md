# kind

[kind](https://kind.sigs.k8s.io/) runs Kubernetes clusters in Docker containers. It's the fastest way to get started with operator-chaos for development and CI pipelines.

## Supported Tiers

| Tier | Supported | Notes |
|------|-----------|-------|
| 1 (PodKill) | Yes | Full support |
| 2 (ConfigDrift, NetworkPartition) | Partial | ConfigDrift works. NetworkPartition creates NetworkPolicies but kind's CNI may not enforce them by default. |
| 3 (CRDMutation, FinalizerBlock, LabelStomping) | Partial | FinalizerBlock and LabelStomping work on any cluster. CRDMutation works if you install the target CRDs. No Routes. |
| 4+ | No | No OLM, no OpenShift webhooks, no SCCs. |

kind is best for **Tier 1 testing**, SDK development, and fuzz testing (which uses a fake client and doesn't need a cluster at all).

## Cluster Setup

### Create a cluster

```bash
kind create cluster --name chaos-test
```

For experiments that need multiple nodes (to test pod scheduling behavior):

```bash
cat <<EOF | kind create cluster --name chaos-test --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
  - role: worker
EOF
```

### Install ODH operator (manual manifests)

kind doesn't have OLM, so install the operator from manifests:

```bash
# Clone the operator repo
git clone https://github.com/opendatahub-io/opendatahub-operator.git
cd opendatahub-operator

# Install CRDs and deploy
make deploy
```

Alternatively, install just the CRDs without deploying the full operator:

```bash
cd opendatahub-operator
make manifests
kubectl apply -f config/crd/
```

### Install operator-chaos

```bash
go install github.com/opendatahub-io/operator-chaos/cmd/operator-chaos@latest
```

## Running Experiments

### Preflight check

```bash
operator-chaos preflight --knowledge knowledge/odh-model-controller.yaml
```

### Run a Tier 1 experiment

```bash
operator-chaos run experiments/odh-model-controller/pod-kill.yaml \
  --knowledge knowledge/odh-model-controller.yaml \
  --max-tier 1 \
  -v
```

### Run a suite (Tier 1 only)

```bash
operator-chaos suite experiments/odh-model-controller/ \
  --knowledge knowledge/odh-model-controller.yaml \
  --max-tier 1 \
  --report-dir /tmp/chaos-results/
```

## CI Integration

kind is well-suited for GitHub Actions and other CI systems:

```yaml
# .github/workflows/chaos.yml
jobs:
  chaos-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: helm/kind-action@v1
        with:
          version: v0.27.0
      - run: go install github.com/opendatahub-io/operator-chaos/cmd/operator-chaos@latest
      - run: |
          operator-chaos suite experiments/ \
            --knowledge-dir knowledge/ \
            --max-tier 1 \
            --report-dir /tmp/chaos-results/
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: chaos-results
          path: /tmp/chaos-results/
```

## Limitations

- **No OLM**: Can't test Subscription, CSV, or InstallPlan scenarios
- **No Routes**: OpenShift Route experiments won't work. Use Ingress-based alternatives if needed.
- **No SCCs**: SecurityContextConstraints don't exist in vanilla Kubernetes
- **CNI**: Default kindnet CNI may not enforce NetworkPolicies. Install Calico if you need Tier 2 NetworkPartition experiments:
  ```bash
  kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.27.0/manifests/calico.yaml
  ```
- **No persistent storage by default**: Experiments involving PVCs may need a storage provisioner

## Next Steps

- [CLI Quickstart](../../modes/cli.md) for running your first experiment
- [CI Integration](../ci-integration.md) for pipeline setup details
- [OCP guide](ocp.md) when you're ready for higher-tier experiments
