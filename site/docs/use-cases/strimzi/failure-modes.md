# Strimzi Failure Modes

## Coverage

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| PodKill | low | cluster-operator/pod-kill.yaml | Killing the cluster-operator pod triggers a Deployment rollout. Kafka clusters remain running. |
| NetworkPartition | medium | cluster-operator/network-partition.yaml | Isolating the cluster-operator from the API server stalls Kafka cluster reconciliation. |
| LabelStomping | high | cluster-operator/label-stomping.yaml | Overwriting a label on the cluster-operator Deployment tests OLM label reconciliation. |
| QuotaExhaustion | high | cluster-operator/quota-exhaustion.yaml | Applying zero-limit ResourceQuota prevents new pod creation. |
| RBACRevoke | high | cluster-operator/rbac-revoke.yaml | Revoking ClusterRoleBinding blocks Kafka resource reconciliation. |
| DeploymentScaleZero | high | cluster-operator/deployment-scale-zero.yaml | Scaling the Deployment to zero replicas. OLM does not restore replicas. |
| LeaderElectionDisrupt | medium | cluster-operator/leader-election-disrupt.yaml | Deleting the leader election Lease forces re-election. |
| ConfigDrift | high | cluster-operator/config-drift.yaml | Corrupting operator configuration tests self-healing. |

## Experiment Details

### cluster-operator

#### cluster-operator-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** cluster-operator

Killing the Strimzi cluster-operator pod triggers a Deployment rollout that recreates it. Existing Kafka clusters remain operational since they run independently.

---

#### cluster-operator-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** cluster-operator

Isolating the cluster-operator from the API server stalls Kafka cluster reconciliation. Existing clusters continue serving traffic. After the partition is lifted, the operator reconnects and processes backlogged reconciliation events.

---

#### cluster-operator-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** cluster-operator

Overwriting a label on the cluster-operator Deployment tests whether OLM detects and restores the label.

---

#### cluster-operator-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** cluster-operator

Applying a zero-limit ResourceQuota to the namespace prevents new pod creation. The chaos framework removes the quota via TTL-based cleanup.

---

#### cluster-operator-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** cluster-operator

Revoking the cluster-operator's ClusterRoleBinding blocks Kafka resource reconciliation. The pod remains running but cannot manage Kafka clusters. After rollback, reconciliation resumes.

---

#### cluster-operator-deployment-scale-zero

- **Type:** DeploymentScaleZero
- **Danger Level:** high
- **Component:** cluster-operator

Scaling the cluster-operator Deployment to zero replicas. OLM does not automatically restore the replica count, making this a Degraded finding.

---

#### cluster-operator-leader-election-disrupt

- **Type:** LeaderElectionDisrupt
- **Danger Level:** medium
- **Component:** cluster-operator

Deleting the leader election Lease forces the cluster-operator to re-acquire leadership. During re-election, reconciliation is temporarily paused.

---

#### cluster-operator-config-drift

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** cluster-operator

Corrupting the operator's configuration tests whether the operator or its controller can detect and restore correct configuration values.


<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
