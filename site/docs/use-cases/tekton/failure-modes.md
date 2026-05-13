# Tekton Failure Modes

## Coverage

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| PodKill | low | pipelines-controller/pod-kill.yaml | Killing the pipelines-controller pod triggers a Deployment rollout. |
| NetworkPartition | medium | pipelines-controller/network-partition.yaml | Isolating the controller from the API server stalls pipeline execution. |
| LabelStomping | high | pipelines-controller/label-stomping.yaml | Overwriting a label on the controller Deployment. |
| QuotaExhaustion | high | pipelines-controller/quota-exhaustion.yaml | Applying zero-limit ResourceQuota prevents new pod creation. |
| RBACRevoke | high | pipelines-controller/rbac-revoke.yaml | Revoking ClusterRoleBinding blocks pipeline reconciliation. |
| DeploymentScaleZero | high | pipelines-controller/deployment-scale-zero.yaml | Scaling controller to zero replicas. |
| ConfigDrift | high | pipelines-controller/config-drift.yaml | Corrupting controller configuration. |
| PodKill | low | pipelines-webhook/pod-kill.yaml | Killing the pipelines-webhook pod triggers a Deployment rollout. |
| NetworkPartition | medium | pipelines-webhook/network-partition.yaml | Isolating the webhook from the API server. |
| LabelStomping | high | pipelines-webhook/label-stomping.yaml | Overwriting a label on the webhook Deployment. |
| QuotaExhaustion | high | pipelines-webhook/quota-exhaustion.yaml | Applying zero-limit ResourceQuota prevents new pod creation. |
| DeploymentScaleZero | high | pipelines-webhook/deployment-scale-zero.yaml | Scaling webhook to zero replicas. |
| ConfigDrift | high | pipelines-webhook/config-drift.yaml | Corrupting webhook configuration. |
| ResourceDeletion | high | pipelines-webhook/service-deletion.yaml | Deleting the webhook Service. |

## Experiment Details

### pipelines-controller

#### pipelines-controller-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** pipelines-controller

Killing the pipelines-controller pod triggers a Deployment rollout. Running PipelineRuns are temporarily stalled but resume after recovery.

---

#### pipelines-controller-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** pipelines-controller

Isolating the controller from the API server stalls pipeline execution. After the partition is lifted, the controller reconnects and resumes processing.

---

#### pipelines-controller-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** pipelines-controller

Overwriting a label on the controller Deployment tests whether the Pipelines operator detects and restores the label.

---

#### pipelines-controller-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** pipelines-controller

Applying a zero-limit ResourceQuota prevents new pod creation. The chaos framework removes the quota via TTL-based cleanup.

---

#### pipelines-controller-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** pipelines-controller

Revoking the controller's ClusterRoleBinding blocks pipeline reconciliation. After rollback, reconciliation resumes.

---

#### pipelines-controller-deployment-scale-zero

- **Type:** DeploymentScaleZero
- **Danger Level:** high
- **Component:** pipelines-controller

Scaling the controller Deployment to zero replicas tests operator restoration behavior.

---

#### pipelines-controller-config-drift

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** pipelines-controller

Corrupting the controller configuration tests self-healing capabilities.

---

### pipelines-webhook

#### pipelines-webhook-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** pipelines-webhook

Killing the webhook pod triggers a Deployment rollout. Pipeline resource admission is temporarily unavailable.

---

#### pipelines-webhook-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** pipelines-webhook

Isolating the webhook from the API server blocks Tekton resource admission.

---

#### pipelines-webhook-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** pipelines-webhook

Overwriting a label on the webhook Deployment tests label reconciliation.

---

#### pipelines-webhook-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** pipelines-webhook

Applying a zero-limit ResourceQuota prevents new webhook pod creation.

---

#### pipelines-webhook-deployment-scale-zero

- **Type:** DeploymentScaleZero
- **Danger Level:** high
- **Component:** pipelines-webhook

Scaling the webhook Deployment to zero replicas tests operator restoration behavior.

---

#### pipelines-webhook-config-drift

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** pipelines-webhook

Corrupting the webhook configuration tests self-healing capabilities.

---

#### pipelines-webhook-service-deletion

- **Type:** ResourceDeletion
- **Danger Level:** high
- **Component:** pipelines-webhook

Deleting the webhook Service. Initial automated run reported Degraded, but manual testing confirmed the Service is recreated within 10s (false positive).


<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
