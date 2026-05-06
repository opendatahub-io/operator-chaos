#!/bin/bash
# Setup kueue test fixtures for domain-specific chaos experiments.
# Prerequisites: Red Hat Build of Kueue operator installed.
# Usage: ./setup.sh [--teardown]
set -euo pipefail

FIXTURES_DIR="$(cd "$(dirname "$0")" && pwd)"

if [[ "${1:-}" == "--teardown" ]]; then
    echo "Tearing down kueue test fixtures..."
    kubectl delete -f "$FIXTURES_DIR/local-queue.yaml" --ignore-not-found
    kubectl delete -f "$FIXTURES_DIR/second-cluster-queue.yaml" --ignore-not-found
    kubectl delete -f "$FIXTURES_DIR/cluster-queue.yaml" --ignore-not-found
    kubectl delete -f "$FIXTURES_DIR/resource-flavor.yaml" --ignore-not-found
    kubectl delete -f "$FIXTURES_DIR/workload-priority-class.yaml" --ignore-not-found
    echo "Teardown complete."
    exit 0
fi

echo "Setting up kueue test fixtures..."

# Create Kueue instance (if not already present)
if ! kubectl get kueues.kueue.openshift.io kueue &>/dev/null; then
    echo "Creating Kueue instance..."
    kubectl apply -f "$FIXTURES_DIR/kueue-instance.yaml"
    echo "Waiting for kueue operand to be ready..."
    kubectl wait --for=condition=Available deployment/kueue-controller-manager \
        -n openshift-kueue-operator --timeout=120s 2>/dev/null || true
fi

kubectl apply -f "$FIXTURES_DIR/resource-flavor.yaml"
kubectl apply -f "$FIXTURES_DIR/workload-priority-class.yaml"
kubectl apply -f "$FIXTURES_DIR/cluster-queue.yaml"
kubectl apply -f "$FIXTURES_DIR/second-cluster-queue.yaml"
kubectl apply -f "$FIXTURES_DIR/local-queue.yaml"

echo "Fixtures ready. Resources created:"
echo "  ResourceFlavor:        chaos-test-flavor"
echo "  ClusterQueue:          chaos-test-cq (cohort: chaos-test-cohort)"
echo "  ClusterQueue:          chaos-test-cq-secondary (cohort: chaos-test-cohort)"
echo "  LocalQueue:            chaos-test-lq -> chaos-test-cq (namespace: chaos-kueue-test)"
echo "  WorkloadPriorityClass: chaos-test-high-priority (1000), chaos-test-low-priority (1)"
