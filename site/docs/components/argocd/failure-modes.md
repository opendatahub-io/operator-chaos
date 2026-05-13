# ArgoCD Failure Modes

## Coverage

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| PodKill | low | server/pod-kill.yaml | Killing the ArgoCD server pod triggers a Deployment rollout. |
| NetworkPartition | medium | server/network-partition.yaml | Isolating the server from the API server stalls sync operations. |
| LabelStomping | high | server/label-stomping.yaml | Overwriting a label on the server Deployment. |
| QuotaExhaustion | high | server/quota-exhaustion.yaml | Applying zero-limit ResourceQuota prevents new pod creation. |
| RBACRevoke | high | server/rbac-revoke.yaml | Revoking ClusterRoleBinding blocks application management. |
| DeploymentScaleZero | high | server/deployment-scale-zero.yaml | Scaling server to zero replicas. |
| LeaderElectionDisrupt | medium | server/leader-election-disrupt.yaml | Disrupting leader election Lease. |
| ConfigDrift | high | server/config-drift.yaml | Corrupting server configuration. |
| PodKill | low | repo-server/pod-kill.yaml | Killing the repo-server pod triggers a Deployment rollout. |
| NetworkPartition | medium | repo-server/network-partition.yaml | Isolating the repo-server from the API server. |
| LabelStomping | high | repo-server/label-stomping.yaml | Overwriting a label on the repo-server Deployment. |
| QuotaExhaustion | high | repo-server/quota-exhaustion.yaml | Applying zero-limit ResourceQuota prevents new pod creation. |
| RBACRevoke | high | repo-server/rbac-revoke.yaml | Revoking ClusterRoleBinding blocks repository access. |
| DeploymentScaleZero | high | repo-server/deployment-scale-zero.yaml | Scaling repo-server to zero replicas. |
| LeaderElectionDisrupt | medium | repo-server/leader-election-disrupt.yaml | Disrupting leader election Lease. |
| ConfigDrift | high | repo-server/config-drift.yaml | Corrupting repo-server configuration. |

## Experiment Details

### server

#### server-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** server

Killing the ArgoCD server pod triggers a Deployment rollout. GitOps sync operations are temporarily paused but resume after recovery.

---

#### server-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** server

Isolating the server from the API server stalls sync operations. Existing deployed resources are not affected. Recovery occurs after the partition is lifted.

---

#### server-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** server

Overwriting a label on the server Deployment tests whether the GitOps operator detects and restores the label.

---

#### server-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** server

Applying a zero-limit ResourceQuota prevents new pod creation. The chaos framework removes the quota via TTL-based cleanup.

---

#### server-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** server

Revoking the server's ClusterRoleBinding blocks application management. After rollback, operations resume.

---

#### server-deployment-scale-zero

- **Type:** DeploymentScaleZero
- **Danger Level:** high
- **Component:** server

Scaling the server Deployment to zero replicas tests OLM restoration behavior.

---

#### server-leader-election-disrupt

- **Type:** LeaderElectionDisrupt
- **Danger Level:** medium
- **Component:** server

Disrupting the leader election Lease forces re-election.

---

#### server-config-drift

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** server

Corrupting the server configuration tests self-healing capabilities.

---

### repo-server

#### repo-server-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** repo-server

Killing the repo-server pod triggers a Deployment rollout. Git repository fetches are temporarily unavailable.

---

#### repo-server-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** repo-server

Isolating the repo-server from the API server and Git repositories stalls manifest generation. Recovery occurs after the partition is lifted.

---

#### repo-server-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** repo-server

Overwriting a label on the repo-server Deployment tests label reconciliation.

---

#### repo-server-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** repo-server

Applying a zero-limit ResourceQuota prevents new pod creation.

---

#### repo-server-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** repo-server

Revoking the repo-server's ClusterRoleBinding blocks repository access.

---

#### repo-server-deployment-scale-zero

- **Type:** DeploymentScaleZero
- **Danger Level:** high
- **Component:** repo-server

Scaling the repo-server Deployment to zero replicas tests OLM restoration behavior.

---

#### repo-server-leader-election-disrupt

- **Type:** LeaderElectionDisrupt
- **Danger Level:** medium
- **Component:** repo-server

Disrupting the leader election Lease forces re-election.

---

#### repo-server-config-drift

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** repo-server

Corrupting the repo-server configuration tests self-healing capabilities.


<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
