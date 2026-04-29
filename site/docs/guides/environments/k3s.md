# k3s

[k3s](https://k3s.io/) is a lightweight Kubernetes distribution that provides real networking, Traefik ingress, and a built-in CNI that enforces NetworkPolicies. This makes it suitable for Tier 1-2 testing with minimal resource overhead.

## Supported Tiers

| Tier | Supported | Notes |
|------|-----------|-------|
| 1 (PodKill) | Yes | Full support |
| 2 (ConfigDrift, NetworkPartition) | Yes | k3s v1.28+ ships a built-in network policy controller (kube-router) alongside Flannel. Older versions need Calico. |
| 3 (CRDMutation, FinalizerBlock, LabelStomping) | Partial | Works if target CRDs are installed. No Routes (use Traefik IngressRoute instead). |
| 4+ | No | No OLM, no OpenShift webhooks, no SCCs |

k3s is best for **Tier 1-2 testing** where you need real network policy enforcement without the overhead of a full OpenShift cluster.

## Cluster Setup

### Single-node cluster

```bash
curl -sfL https://get.k3s.io | sh -
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
```

### Multi-node cluster (with k3d)

[k3d](https://k3d.io/) wraps k3s in Docker containers, similar to kind but with k3s features:

```bash
# Install k3d
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

# Create a multi-node cluster
k3d cluster create chaos-test --agents 2

# Kubeconfig is automatically merged
kubectl get nodes
```

### Install ODH operator

k3s doesn't include OLM. Install the operator from manifests:

```bash
git clone https://github.com/opendatahub-io/opendatahub-operator.git
cd opendatahub-operator
make deploy
```

Or install just the CRDs for targeted testing:

```bash
kubectl apply -f config/crd/bases/
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

### Run Tier 1-2 experiments

```bash
# PodKill (Tier 1)
operator-chaos run experiments/odh-model-controller/pod-kill.yaml \
  --knowledge knowledge/odh-model-controller.yaml -v

# NetworkPartition (Tier 2) - works because k3s enforces NetworkPolicies
operator-chaos run experiments/odh-model-controller/network-partition.yaml \
  --knowledge knowledge/odh-model-controller.yaml -v
```

### Run a filtered suite

```bash
operator-chaos suite experiments/odh-model-controller/ \
  --knowledge knowledge/odh-model-controller.yaml \
  --max-tier 2 \
  --report-dir /tmp/chaos-results/
```

## Advantages over kind

- **NetworkPolicy enforcement**: k3s v1.28+ ships a built-in kube-router-based policy controller, so NetworkPartition experiments work without extra setup (older versions need Calico)
- **Traefik ingress**: Built-in ingress controller for testing service accessibility
- **Lower memory footprint**: k3s uses ~512MB vs ~1GB for a comparable kind cluster
- **Faster startup**: Single-node k3s starts in under 30 seconds

## Limitations

- **No OLM**: Can't test Subscription, CSV, or InstallPlan scenarios
- **No Routes**: OpenShift Route experiments won't work. Traefik IngressRoute is the closest alternative.
- **No SCCs**: SecurityContextConstraints don't exist
- **No webhook certificates**: k3s doesn't provide the cert-manager integration that OpenShift does, so webhook experiments may need manual cert setup
- **Single-node by default**: Use k3d for multi-node testing

## Next Steps

- [CLI Quickstart](../../modes/cli.md) for the full experiment workflow
- [OCP guide](ocp.md) when you need OLM, Routes, and higher-tier experiments
- [CI Integration](../ci-integration.md) for pipeline setup with k3s/k3d
