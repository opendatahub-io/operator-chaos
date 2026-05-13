# Prometheus Operator Failure Modes

## Coverage

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| PodKill | low | prometheus-operator/pod-kill.yaml | Killing the prometheus-operator pod triggers a Deployment rollout. |
| NetworkPartition | medium | prometheus-operator/network-partition.yaml | Isolating the operator from the API server stalls Prometheus reconciliation. |
| LabelStomping | high | prometheus-operator/label-stomping.yaml | Overwriting a label on the operator Deployment. |
| QuotaExhaustion | high | prometheus-operator/quota-exhaustion.yaml | Applying zero-limit ResourceQuota prevents new pod creation. |
| RBACRevoke | high | prometheus-operator/rbac-revoke.yaml | Revoking ClusterRoleBinding blocks Prometheus resource management. |
| DeploymentScaleZero | high | prometheus-operator/deployment-scale-zero.yaml | Scaling operator to zero replicas. |

## Experiment Details

### prometheus-operator

#### prometheus-operator-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** prometheus-operator

Killing the prometheus-operator pod triggers a Deployment rollout. Running Prometheus and Alertmanager instances continue serving metrics. Reconciliation of ServiceMonitor and PrometheusRule changes stalls until the operator recovers.

---

#### prometheus-operator-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** prometheus-operator

Isolating the operator from the API server stalls Prometheus configuration reconciliation. Existing Prometheus instances continue scraping and serving queries. After the partition is lifted, the operator reconnects and processes backlogged events.

---

#### prometheus-operator-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** prometheus-operator

Overwriting a label on the operator Deployment tests whether cluster-monitoring-operator detects and restores the label.

---

#### prometheus-operator-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** prometheus-operator

Applying a zero-limit ResourceQuota prevents new pod creation. The chaos framework removes the quota via TTL-based cleanup.

---

#### prometheus-operator-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** prometheus-operator

Revoking the operator's ClusterRoleBinding blocks Prometheus and Alertmanager resource management. After rollback, reconciliation resumes.

---

#### prometheus-operator-deployment-scale-zero

- **Type:** DeploymentScaleZero
- **Danger Level:** high
- **Component:** prometheus-operator

Scaling the operator Deployment to zero replicas. cluster-monitoring-operator restores the replica count.


<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
