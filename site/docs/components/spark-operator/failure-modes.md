# Spark Operator Failure Modes

## Coverage

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| PodKill | low | controller/pod-kill.yaml | Killing the controller pod triggers a Deployment rollout. |
| NetworkPartition | medium | controller/network-partition.yaml | Isolating the controller from the API server. Controller becomes permanently non-functional. |
| LabelStomping | high | controller/label-stomping.yaml | Overwriting a label on the controller Deployment. |
| QuotaExhaustion | high | controller/quota-exhaustion.yaml | Applying zero-limit ResourceQuota prevents new pod creation. |
| RBACRevoke | high | controller/rbac-revoke.yaml | Revoking ClusterRoleBinding blocks SparkApplication reconciliation. |
| DeploymentScaleZero | high | controller/deployment-scale-zero.yaml | Scaling controller to zero replicas. Not restored automatically. |
| PodKill | low | webhook/pod-kill.yaml | Killing the webhook pod triggers a Deployment rollout. |
| NetworkPartition | medium | webhook/network-partition.yaml | Isolating the webhook from the API server. |
| LabelStomping | high | webhook/label-stomping.yaml | Overwriting a label on the webhook Deployment. |
| QuotaExhaustion | high | webhook/quota-exhaustion.yaml | Applying zero-limit ResourceQuota prevents new pod creation. |
| DeploymentScaleZero | high | webhook/deployment-scale-zero.yaml | Scaling webhook to zero replicas. Not restored automatically. |
| ConfigDrift | high | webhook/webhook-cert-corrupt.yaml | Corrupting the webhook TLS certificate. |

## Experiment Details

### controller

#### controller-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** controller

Killing the Spark operator controller pod triggers a Deployment rollout that recreates it. Running SparkApplications are not affected.

---

#### controller-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** controller

Isolating the controller from the API server causes the controller to become permanently non-functional. This is a verified Failed finding: the controller does not recover connectivity after the partition is lifted without a pod restart.

---

#### controller-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** controller

Overwriting a label on the controller Deployment tests whether the managing controller detects and restores the label.

---

#### controller-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** controller

Applying a zero-limit ResourceQuota prevents new pod creation. The chaos framework removes the quota via TTL-based cleanup.

---

#### controller-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** controller

Revoking the controller's ClusterRoleBinding blocks SparkApplication reconciliation. The pod remains running but cannot manage resources. After rollback, reconciliation resumes.

---

#### controller-deployment-scale-zero

- **Type:** DeploymentScaleZero
- **Danger Level:** high
- **Component:** controller

Scaling the controller Deployment to zero replicas. Since Spark operator is Helm/kustomize-managed, there is no controller to restore replicas automatically.

---

### webhook

#### webhook-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** webhook

Killing the webhook pod triggers a Deployment rollout. SparkApplication validation is temporarily unavailable.

---

#### webhook-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** webhook

Isolating the webhook from the API server blocks SparkApplication admission. Recovery occurs after the partition is lifted.

---

#### webhook-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** webhook

Overwriting a label on the webhook Deployment tests label reconciliation.

---

#### webhook-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** webhook

Applying a zero-limit ResourceQuota prevents new webhook pod creation.

---

#### webhook-deployment-scale-zero

- **Type:** DeploymentScaleZero
- **Danger Level:** high
- **Component:** webhook

Scaling the webhook Deployment to zero replicas. Not restored automatically.

---

#### webhook-cert-corrupt

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** webhook

Corrupting the webhook TLS certificate tests whether the operator or cert-manager can regenerate it.


<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
