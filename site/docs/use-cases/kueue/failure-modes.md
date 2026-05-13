# kueue Failure Modes

## Coverage

This document covers 30 experiments across three deployment models:

- **Legacy (Root):** 5 experiments for DSC-managed Kueue (RHOAI 2.x / ODH)
- **kueue-operator:** 11 experiments for the OLM-managed Kueue operator
- **kueue-operand:** 14 experiments for Kueue workload admission resources

### Legacy (Root) Experiments

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| FinalizerBlock | low | finalizer-block.yaml | When a stuck finalizer prevents a Workload from being deleted, the controller sh... |
| NetworkPartition | medium | network-partition.yaml | When kueue-controller-manager pods are network-partitioned from the API server, ... |
| PodKill | low | pod-kill.yaml | When the kueue-controller-manager pod is killed, pending workloads should queue ... |
| RBACRevoke | high | rbac-revoke.yaml | When the kueue ClusterRoleBinding subjects are revoked, the controller can no lo... |
| WebhookDisrupt | high | webhook-disrupt.yaml | When the kueue validating webhook failurePolicy is weakened from Fail to Ignore,... |

### kueue-operator Experiments

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| CRDMutation | high | crd-mutation.yaml | Corrupting the Kueue CRD's singular name should cause API server confusion for n... |
| CRDMutation | high | deployment-scale-zero.yaml | Scaling the kueue operator deployment to zero replicas removes all operator pods... |
| LabelStomping | high | label-stomping.yaml | When a label used for resource discovery is overwritten on the kueue-operator De... |
| CRDMutation | high | leader-lease-corrupt.yaml | When the leader lease holder identity is corrupted, the kueue-operator should de... |
| NetworkPartition | medium | network-partition.yaml | When kueue-operator pods are network-isolated via a deny-all NetworkPolicy, the ... |
| CRDMutation | high | olm-csv-owned-crd-corrupt.yaml | Corrupting the CSV's owned CRD list to an empty array should cause OLM to report... |
| CRDMutation | medium | olm-subscription-approval-flip.yaml | Flipping installPlanApproval from Automatic to Manual should block any pending u... |
| CRDMutation | medium | olm-subscription-channel-corrupt.yaml | Mutating the Subscription channel to a non-existent value should cause OLM to re... |
| OwnerRefOrphan | high | ownerref-orphan.yaml | When owner references are removed from the kueue-operator Deployment, the operat... |
| PodKill | low | pod-kill.yaml | When a kueue-operator pod is killed, the Deployment controller should restart it... |
| QuotaExhaustion | high | quota-exhaustion.yaml | When a ResourceQuota with zero limits is applied to openshift-kueue-operator, no... |
| RBACRevoke | high | rbac-revoke.yaml | When the kueue-operator ClusterRoleBinding subjects are revoked, the operator lo... |

### kueue-operand Experiments

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| PodKill | high | all-pods-kill.yaml | Killing all kueue controller pods simultaneously (both replicas) causes complete... |
| CRDMutation | high | clusterqueue-borrowing-limit-zero.yaml | Setting all borrowingLimits to zero while the ClusterQueue is in a cohort should... |
| CRDMutation | high | clusterqueue-cohort-detach.yaml | Pointing a ClusterQueue's cohort to a nonexistent name while it shares quota wit... |
| CRDMutation | high | clusterqueue-namespace-selector-corrupt.yaml | Corrupting the ClusterQueue's namespaceSelector to match no namespaces should pr... |
| CRDMutation | high | clusterqueue-quota-corrupt.yaml | Corrupting a ClusterQueue's resourceGroups to an empty array removes all quota d... |
| CRDMutation | high | clusterqueue-stop-policy.yaml | Setting a ClusterQueue's stopPolicy to HoldAndDrain should immediately stop new ... |
| ConfigDrift | high | configmap-corrupt.yaml | Corrupting the kueue-manager-config ConfigMap with invalid content tests crash r... |
| CRDMutation | high | controller-scale-zero.yaml | Scaling the kueue controller-manager to zero pods removes all workload admission... |
| CRDMutation | high | fair-sharing-weight-zero.yaml | Setting a ClusterQueue's fairSharing weight to zero could cause a division-by-ze... |
| CRDMutation | high | kueue-cr-loglevel-corrupt.yaml | Setting the Kueue CR's logLevel to TraceAll forces maximum verbosity on the oper... |
| CRDMutation | medium | localqueue-stop-policy.yaml | Setting a LocalQueue's stopPolicy to HoldAndDrain stops new workload admissions ... |
| CRDMutation | high | priority-inversion.yaml | Inverting the priority ordering by setting the high-priority class value to 1 (s... |
| CRDMutation | high | resourceflavor-delete-in-use.yaml | Corrupting a ResourceFlavor that is actively referenced by ClusterQueues tests w... |
| ConfigDrift | high | webhook-cert-corrupt.yaml | Corrupting the kueue webhook TLS certificate should cause API server webhook cal... |

## Experiment Details

## Legacy (Root) Experiments

### kueue-finalizer-block

- **Type:** FinalizerBlock
- **Danger Level:** low
- **Component:** kueue-controller-manager

When a stuck finalizer prevents a Workload from being deleted, the controller should handle the Terminating state gracefully and not leak associated resource quota reservations. The chaos framework removes the finalizer via TTL-based cleanup after 300s.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-finalizer-block
spec:
  tier: 3
  target:
    operator: kueue
    component: kueue-controller-manager
    resource: Workload
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kueue-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: FinalizerBlock
    parameters:
      apiVersion: kueue.x-k8s.io/v1beta1
      kind: Workload
      name: test-workload
      finalizer: kueue.x-k8s.io/managed-resources
    ttl: "300s"
  hypothesis:
    description: >-
      When a stuck finalizer prevents a Workload from being deleted, the
      controller should handle the Terminating state gracefully and not
      leak associated resource quota reservations. The chaos framework
      removes the finalizer via TTL-based cleanup after 300s.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>

---

### kueue-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** kueue-controller-manager

When kueue-controller-manager pods are network-partitioned from the API server, workload admission stops and no new workloads are scheduled. Existing admitted workloads continue running. Once the partition is removed, scheduling resumes without manual intervention.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-network-partition
spec:
  tier: 2
  target:
    operator: kueue
    component: kueue-controller-manager
    resource: Deployment/kueue-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kueue-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      labelSelector: control-plane=controller-manager,app.kubernetes.io/name=kueue
    ttl: "300s"
  hypothesis:
    description: >-
      When kueue-controller-manager pods are network-partitioned from the
      API server, workload admission stops and no new workloads are
      scheduled. Existing admitted workloads continue running. Once the
      partition is removed, scheduling resumes without manual intervention.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>

---

### kueue-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** kueue-controller-manager

When the kueue-controller-manager pod is killed, pending workloads should queue but not be admitted during downtime. Kubernetes should recreate the pod, and the controller should recover and resume scheduling within the recovery timeout.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-pod-kill
spec:
  tier: 1
  target:
    operator: kueue
    component: kueue-controller-manager
    resource: Deployment/kueue-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kueue-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: control-plane=controller-manager,app.kubernetes.io/name=kueue
    count: 1
    ttl: "300s"
  hypothesis:
    description: >-
      When the kueue-controller-manager pod is killed, pending workloads
      should queue but not be admitted during downtime. Kubernetes should
      recreate the pod, and the controller should recover and resume
      scheduling within the recovery timeout.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - opendatahub
```

</details>

---

### kueue-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** kueue-controller-manager

When the kueue ClusterRoleBinding subjects are revoked, the controller can no longer read or update Workloads, ClusterQueues, or LocalQueues. Admission stops with 403 errors. Once permissions are restored, normal scheduling resumes without restart.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-rbac-revoke
spec:
  tier: 4
  target:
    operator: kueue
    component: kueue-controller-manager
    resource: ClusterRoleBinding/kueue-controller-manager-rolebinding
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kueue-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: RBACRevoke
    dangerLevel: high
    parameters:
      bindingName: kueue-controller-manager-rolebinding
      bindingType: ClusterRoleBinding
    ttl: "60s"
  hypothesis:
    description: >-
      When the kueue ClusterRoleBinding subjects are revoked, the controller
      can no longer read or update Workloads, ClusterQueues, or LocalQueues.
      Admission stops with 403 errors. Once permissions are restored, normal
      scheduling resumes without restart.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

### kueue-webhook-disrupt

- **Type:** WebhookDisrupt
- **Danger Level:** high
- **Component:** kueue-controller-manager

When the kueue validating webhook failurePolicy is weakened from Fail to Ignore, invalid Workload and ClusterQueue specs can be submitted bypassing validation. The controller should handle invalid resources gracefully. The chaos framework restores the original failurePolicy via TTL-based cleanup after 60s.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-webhook-disrupt
spec:
  tier: 4
  target:
    operator: kueue
    component: kueue-controller-manager
    resource: ValidatingWebhookConfiguration/vworkload.kb.io
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kueue-controller-manager
        namespace: opendatahub
        conditionType: Available
    timeout: "30s"
  injection:
    type: WebhookDisrupt
    dangerLevel: high
    parameters:
      webhookName: vworkload.kb.io
      action: setFailurePolicy
      value: Ignore
    ttl: "60s"
  hypothesis:
    description: >-
      When the kueue validating webhook failurePolicy is weakened from Fail
      to Ignore, invalid Workload and ClusterQueue specs can be submitted
      bypassing validation. The controller should handle invalid resources
      gracefully. The chaos framework restores the original failurePolicy
      via TTL-based cleanup after 60s.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

## kueue-operator Experiments

### kueue-operator-crd-mutation

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operator

Corrupting the Kueue CRD's singular name should cause API server confusion for new resource lookups. The operator deployment should remain running and existing resources should continue functioning. After rollback, the CRD names are restored and normal API access resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-crd-mutation
spec:
  tier: 3
  target:
    operator: kueue-operator
    component: kueue-operator
    resource: CRD/kueues.kueue.openshift.io
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    parameters:
      apiVersion: "apiextensions.k8s.io/v1"
      kind: "CustomResourceDefinition"
      name: "kueues.kueue.openshift.io"
      path: "spec.names.singular"
      value: "corrupted-by-chaos"
    dangerLevel: high
    ttl: "300s"
  hypothesis:
    description: >-
      Corrupting the Kueue CRD's singular name should cause API server
      confusion for new resource lookups. The operator deployment should
      remain running and existing resources should continue functioning.
      After rollback, the CRD names are restored and normal API access
      resumes.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

### kueue-operator-deployment-scale-zero

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operator

Scaling the kueue operator deployment to zero replicas removes all operator pods. OLM should detect the unavailable deployment and potentially restart it. Without the operator, no reconciliation of the Kueue CR occurs, but existing operand (kueue-controller-manager) continues running. After rollback, replicas are restored and the operator resumes reconciliation.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-deployment-scale-zero
spec:
  tier: 5
  target:
    operator: rh-kueue
    component: kueue-operator
    resource: Deployment/openshift-kueue-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    dangerLevel: high
    parameters:
      apiVersion: "apps/v1"
      kind: "Deployment"
      name: "openshift-kueue-operator"
      namespace: "openshift-kueue-operator"
      path: "spec.replicas"
      value: "0"
    ttl: "120s"
  hypothesis:
    description: >-
      Scaling the kueue operator deployment to zero replicas removes all
      operator pods. OLM should detect the unavailable deployment and
      potentially restart it. Without the operator, no reconciliation of
      the Kueue CR occurs, but existing operand (kueue-controller-manager)
      continues running. After rollback, replicas are restored and the
      operator resumes reconciliation.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-operator-label-stomping

- **Type:** LabelStomping
- **Danger Level:** high
- **Component:** kueue-operator

When a label used for resource discovery is overwritten on the kueue-operator Deployment, the operator should detect the label drift and restore the correct label value.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-label-stomping
spec:
  tier: 3
  target:
    operator: rh-kueue
    component: kueue-operator
    resource: Deployment/openshift-kueue-operator
  steadyState:
    checks:
      - type: resourceExists
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
    timeout: "30s"
  injection:
    type: LabelStomping
    dangerLevel: high
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: openshift-kueue-operator
      labelKey: app.kubernetes.io/name
      action: overwrite
    ttl: "300s"
  hypothesis:
    description: >-
      When a label used for resource discovery is overwritten on the
      kueue-operator Deployment, the operator should detect the label drift
      and restore the correct label value.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-operator-leader-lease-corrupt

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operator

When the leader lease holder identity is corrupted, the kueue-operator should detect the lease conflict and re-acquire leadership.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-leader-lease-corrupt
spec:
  tier: 3
  target:
    operator: rh-kueue
    component: kueue-operator
    resource: Lease/openshift-kueue-operator-lock
  steadyState:
    checks:
      - type: resourceExists
        apiVersion: coordination.k8s.io/v1
        kind: Lease
        name: openshift-kueue-operator-lock
        namespace: openshift-kueue-operator
    timeout: "30s"
  injection:
    type: CRDMutation
    dangerLevel: high
    parameters:
      apiVersion: "coordination.k8s.io/v1"
      kind: "Lease"
      name: openshift-kueue-operator-lock
      field: "holderIdentity"
      value: "chaos-fake-leader"
    ttl: "120s"
  hypothesis:
    description: >-
      When the leader lease holder identity is corrupted, the kueue-operator
      should detect the lease conflict and re-acquire leadership.
    recoveryTimeout: 60s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-operator-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** kueue-operator

When kueue-operator pods are network-isolated via a deny-all NetworkPolicy, the component should detect the partition and recover once connectivity is restored.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-network-partition
spec:
  tier: 2
  target:
    operator: rh-kueue
    component: kueue-operator
    resource: Deployment/openshift-kueue-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      labelSelector: name=openshift-kueue-operator
    ttl: "60s"
  hypothesis:
    description: >-
      When kueue-operator pods are network-isolated via a deny-all NetworkPolicy,
      the component should detect the partition and recover once connectivity
      is restored.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-operator-olm-csv-owned-crd-corrupt

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operator

Corrupting the CSV's owned CRD list to an empty array should cause OLM to report the CSV as invalid. The operator deployment should remain running since OLM does not delete pods when CSV metadata is corrupted. After rollback, the CSV should return to Succeeded.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-olm-csv-owned-crd-corrupt
spec:
  tier: 3
  target:
    operator: kueue-operator
    component: kueue-operator
    resource: ClusterServiceVersion/kueue-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    parameters:
      apiVersion: "operators.coreos.com/v1alpha1"
      kind: "ClusterServiceVersion"
      name: "kueue-operator.v1.3.1"
      namespace: "openshift-kueue-operator"
      path: "spec.customresourcedefinitions.owned"
      value: "[]"
    dangerLevel: high
    ttl: "300s"
  hypothesis:
    description: >-
      Corrupting the CSV's owned CRD list to an empty array should cause
      OLM to report the CSV as invalid. The operator deployment should
      remain running since OLM does not delete pods when CSV metadata
      is corrupted. After rollback, the CSV should return to Succeeded.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-operator-olm-subscription-approval-flip

- **Type:** CRDMutation
- **Danger Level:** medium
- **Component:** kueue-operator

Flipping installPlanApproval from Automatic to Manual should block any pending upgrades but not affect the currently running operator. The operator deployment should remain available. After rollback, the approval mode is restored to Automatic and OLM resumes auto-approving InstallPlans.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-olm-subscription-approval-flip
spec:
  tier: 3
  target:
    operator: kueue-operator
    component: kueue-operator
    resource: Subscription/kueue-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    parameters:
      apiVersion: "operators.coreos.com/v1alpha1"
      kind: "Subscription"
      name: "kueue-operator"
      namespace: "openshift-kueue-operator"
      path: "spec.installPlanApproval"
      value: "Manual"
    ttl: "300s"
  hypothesis:
    description: >-
      Flipping installPlanApproval from Automatic to Manual should block
      any pending upgrades but not affect the currently running operator.
      The operator deployment should remain available. After rollback,
      the approval mode is restored to Automatic and OLM resumes
      auto-approving InstallPlans.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-operator-olm-subscription-channel-corrupt

- **Type:** CRDMutation
- **Danger Level:** medium
- **Component:** kueue-operator

Mutating the Subscription channel to a non-existent value should cause OLM to report the Subscription as unhealthy. The operator deployment should remain running (channel changes only affect future upgrades, not the currently installed CSV). After rollback, the Subscription channel is restored and OLM resumes normal operation.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-olm-subscription-channel-corrupt
spec:
  tier: 3
  target:
    operator: kueue-operator
    component: kueue-operator
    resource: Subscription/kueue-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    parameters:
      apiVersion: "operators.coreos.com/v1alpha1"
      kind: "Subscription"
      name: "kueue-operator"
      namespace: "openshift-kueue-operator"
      path: "spec.channel"
      value: "nonexistent-channel"
    ttl: "300s"
  hypothesis:
    description: >-
      Mutating the Subscription channel to a non-existent value should cause
      OLM to report the Subscription as unhealthy. The operator deployment
      should remain running (channel changes only affect future upgrades,
      not the currently installed CSV). After rollback, the Subscription
      channel is restored and OLM resumes normal operation.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-operator-ownerref-orphan

- **Type:** OwnerRefOrphan
- **Danger Level:** high
- **Component:** kueue-operator

When owner references are removed from the kueue-operator Deployment, the operator should detect the orphaned resource and re-establish ownership.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-ownerref-orphan
spec:
  tier: 3
  target:
    operator: rh-kueue
    component: kueue-operator
    resource: Deployment/openshift-kueue-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: OwnerRefOrphan
    dangerLevel: high
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: openshift-kueue-operator
    ttl: "120s"
  hypothesis:
    description: >-
      When owner references are removed from the kueue-operator Deployment,
      the operator should detect the orphaned resource and re-establish
      ownership.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-operator-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** kueue-operator

When a kueue-operator pod is killed, the Deployment controller should restart it and the component should recover within the recovery timeout.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-pod-kill
spec:
  tier: 1
  target:
    operator: rh-kueue
    component: kueue-operator
    resource: Deployment/openshift-kueue-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: name=openshift-kueue-operator
    ttl: "30s"
  hypothesis:
    description: >-
      When a kueue-operator pod is killed, the Deployment controller should
      restart it and the component should recover within the recovery timeout.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-operator-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** high
- **Component:** kueue-operator

When a ResourceQuota with zero limits is applied to openshift-kueue-operator, no new pods can be created. The kueue-operator should handle quota exhaustion gracefully. The chaos framework removes the quota via TTL-based cleanup.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-quota-exhaustion
spec:
  tier: 5
  target:
    operator: rh-kueue
    component: kueue-operator
    resource: ResourceQuota/chaos-quota-kueue-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: QuotaExhaustion
    dangerLevel: high
    parameters:
      quotaName: chaos-quota-kueue-operator
      pods: "0"
      cpu: "0"
      memory: "0"
    ttl: "60s"
  hypothesis:
    description: >-
      When a ResourceQuota with zero limits is applied to openshift-kueue-operator,
      no new pods can be created. The kueue-operator should handle quota
      exhaustion gracefully. The chaos framework removes the quota via
      TTL-based cleanup.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-operator-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** kueue-operator

When the kueue-operator ClusterRoleBinding subjects are revoked, the operator loses permissions to manage Kueue CRs and operand resources. OLM should detect the drift and restore the ClusterRoleBinding. After restoration, the operator should resume normal operation.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-operator-rbac-revoke
spec:
  tier: 4
  target:
    operator: kueue-operator
    component: kueue-operator
    resource: ClusterRoleBinding/kueue-operator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: RBACRevoke
    dangerLevel: high
    parameters:
      # NOTE: The CRB name includes an OLM-generated hash suffix that changes
      # per installation. Use --set to override:
      #   --set 'injection.parameters.bindingName=kueue-operator.v1.3.1-<hash>'
      # Find it with: oc get clusterrolebinding | grep kueue
      bindingName: "kueue-operator.v1.3.1-adIoh9OBFFYUD2GRfbZ3KnOAExhQw5kNCMMg3L"
      bindingType: ClusterRoleBinding
    ttl: "60s"
  hypothesis:
    description: >-
      When the kueue-operator ClusterRoleBinding subjects are revoked, the
      operator loses permissions to manage Kueue CRs and operand resources.
      OLM should detect the drift and restore the ClusterRoleBinding.
      After restoration, the operator should resume normal operation.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

## kueue-operand Experiments

### kueue-all-pods-kill

- **Type:** PodKill
- **Danger Level:** high
- **Component:** kueue-operand

Killing all kueue controller pods simultaneously (both replicas) causes complete loss of the workload admission control plane. The Deployment controller should recreate both pods. During the outage window, pending workloads are not admitted but existing admitted workloads continue running. After recovery, leader election completes and admission resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-all-pods-kill
spec:
  tier: 4
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: Deployment/kueue-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kueue-controller-manager
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill
    parameters:
      labelSelector: control-plane=controller-manager
    count: 2
    ttl: "120s"
  hypothesis:
    description: >-
      Killing all kueue controller pods simultaneously (both replicas)
      causes complete loss of the workload admission control plane.
      The Deployment controller should recreate both pods. During the
      outage window, pending workloads are not admitted but existing
      admitted workloads continue running. After recovery, leader
      election completes and admission resumes.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-clusterqueue-borrowing-limit-zero

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operand

Setting all borrowingLimits to zero while the ClusterQueue is in a cohort should prevent all quota borrowing. The controller should detect the policy tightening and handle it gracefully. After rollback, normal borrowing behavior resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-clusterqueue-borrowing-limit-zero
spec:
  tier: 2
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: ClusterQueue/chaos-test-cq
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    dangerLevel: high
    parameters:
      apiVersion: "kueue.x-k8s.io/v1beta1"
      kind: "ClusterQueue"
      name: "chaos-test-cq"
      path: "spec.resourceGroups"
      value: '[{"coveredResources":["cpu","memory"],"flavors":[{"name":"chaos-test-flavor","resources":[{"name":"cpu","nominalQuota":"4","borrowingLimit":"0"},{"name":"memory","nominalQuota":"8Gi","borrowingLimit":"0"}]}]}]'
    ttl: "300s"
  hypothesis:
    description: >-
      Setting all borrowingLimits to zero while the ClusterQueue is in a
      cohort should prevent all quota borrowing. The controller should
      detect the policy tightening and handle it gracefully. After rollback,
      normal borrowing behavior resumes.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

### kueue-clusterqueue-cohort-detach

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operand

Pointing a ClusterQueue's cohort to a nonexistent name while it shares quota with other queues should break the borrowing relationship. The queue should function standalone with only its nominal quota. After rollback, cohort membership is restored and borrowing resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-clusterqueue-cohort-detach
spec:
  tier: 3
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: ClusterQueue/chaos-test-cq
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    parameters:
      apiVersion: "kueue.x-k8s.io/v1beta1"
      kind: "ClusterQueue"
      name: "chaos-test-cq"
      path: "spec.cohort"
      value: "nonexistent-cohort"
    dangerLevel: high
    ttl: "300s"
  hypothesis:
    description: >-
      Pointing a ClusterQueue's cohort to a nonexistent name while it
      shares quota with other queues should break the borrowing relationship.
      The queue should function standalone with only its nominal quota.
      After rollback, cohort membership is restored and borrowing resumes.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

### kueue-clusterqueue-namespace-selector-corrupt

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operand

Corrupting the ClusterQueue's namespaceSelector to match no namespaces should prevent all LocalQueues from being admitted. The controller should handle this gracefully and report the queue as having no matching namespaces. After rollback, normal namespace matching resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-clusterqueue-namespace-selector-corrupt
spec:
  tier: 3
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: ClusterQueue/chaos-test-cq
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    dangerLevel: high
    parameters:
      apiVersion: "kueue.x-k8s.io/v1beta1"
      kind: "ClusterQueue"
      name: "chaos-test-cq"
      path: "spec.namespaceSelector"
      value: "{\"matchLabels\":{\"nonexistent-label\":\"true\"}}"
    ttl: "300s"
  hypothesis:
    description: >-
      Corrupting the ClusterQueue's namespaceSelector to match no namespaces
      should prevent all LocalQueues from being admitted. The controller
      should handle this gracefully and report the queue as having no
      matching namespaces. After rollback, normal namespace matching resumes.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

### kueue-clusterqueue-quota-corrupt

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operand

Corrupting a ClusterQueue's resourceGroups to an empty array removes all quota definitions. The kueue controller should detect the invalid config, stop admitting workloads to this queue, and report the queue as inactive. After rollback, quota is restored and admission resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-clusterqueue-quota-corrupt
spec:
  tier: 3
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: ClusterQueue/chaos-test-cq
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    parameters:
      apiVersion: "kueue.x-k8s.io/v1beta1"
      kind: "ClusterQueue"
      name: "chaos-test-cq"
      path: "spec.resourceGroups"
      value: "[]"
    dangerLevel: high
    ttl: "300s"
  hypothesis:
    description: >-
      Corrupting a ClusterQueue's resourceGroups to an empty array removes
      all quota definitions. The kueue controller should detect the invalid
      config, stop admitting workloads to this queue, and report the queue
      as inactive. After rollback, quota is restored and admission resumes.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

### kueue-clusterqueue-stop-policy

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operand

Setting a ClusterQueue's stopPolicy to HoldAndDrain should immediately stop new workload admissions and evict admitted workloads. The kueue controller should drain gracefully without crashing. After rollback (stopPolicy removed), admission resumes and evicted workloads are re-queued.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-clusterqueue-stop-policy
spec:
  tier: 2
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: ClusterQueue/chaos-test-cq
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    parameters:
      apiVersion: "kueue.x-k8s.io/v1beta1"
      kind: "ClusterQueue"
      name: "chaos-test-cq"
      path: "spec.stopPolicy"
      value: "HoldAndDrain"
    dangerLevel: high
    ttl: "300s"
  hypothesis:
    description: >-
      Setting a ClusterQueue's stopPolicy to HoldAndDrain should immediately
      stop new workload admissions and evict admitted workloads. The kueue
      controller should drain gracefully without crashing. After rollback
      (stopPolicy removed), admission resumes and evicted workloads are
      re-queued.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

### kueue-configmap-corrupt

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** kueue-operand

Corrupting the kueue-manager-config ConfigMap with invalid content tests crash resilience. The controller manager should either continue running with cached config or restart and fall back to defaults. It should not enter CrashLoopBackOff. After rollback, normal configuration is restored.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-configmap-corrupt
spec:
  tier: 3
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: ConfigMap/kueue-manager-config
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: ConfigDrift
    dangerLevel: high
    parameters:
      name: kueue-manager-config
      namespace: openshift-kueue-operator
      key: controller_manager_config.yaml
      value: "corrupted-by-chaos"
      resourceType: ConfigMap
    ttl: "300s"
  hypothesis:
    description: >-
      Corrupting the kueue-manager-config ConfigMap with invalid content
      tests crash resilience. The controller manager should either continue
      running with cached config or restart and fall back to defaults.
      It should not enter CrashLoopBackOff. After rollback, normal
      configuration is restored.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-controller-scale-zero

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operand

Scaling the kueue controller-manager to zero pods removes all workload admission capacity. Pending workloads should stall, no new workloads should be admitted. The kueue operator should detect the unavailable operand deployment and restore replicas via reconciliation. This tests the operator's ability to self-heal its operand. After rollback or operator reconciliation, admission resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-controller-scale-zero
spec:
  tier: 5
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: Deployment/kueue-controller-manager
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kueue-controller-manager
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "60s"
  injection:
    type: CRDMutation
    dangerLevel: high
    parameters:
      apiVersion: "apps/v1"
      kind: "Deployment"
      name: "kueue-controller-manager"
      namespace: "openshift-kueue-operator"
      path: "spec.replicas"
      value: "0"
    ttl: "120s"
  hypothesis:
    description: >-
      Scaling the kueue controller-manager to zero pods removes all
      workload admission capacity. Pending workloads should stall, no
      new workloads should be admitted. The kueue operator should detect
      the unavailable operand deployment and restore replicas via
      reconciliation. This tests the operator's ability to self-heal
      its operand. After rollback or operator reconciliation, admission
      resumes.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>

---

### kueue-fair-sharing-weight-zero

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operand

Setting a ClusterQueue's fairSharing weight to zero could cause a division-by-zero in the fair sharing algorithm (known upstream issue 2473). The controller should either reject the invalid weight or handle it gracefully without panicking. After rollback, normal fair sharing resumes with the original weight.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-fair-sharing-weight-zero
spec:
  tier: 3
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: ClusterQueue/chaos-test-cq
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    parameters:
      apiVersion: "kueue.x-k8s.io/v1beta1"
      kind: "ClusterQueue"
      name: "chaos-test-cq"
      path: "spec.fairSharing.weight"
      value: "0"
    dangerLevel: high
    ttl: "300s"
  hypothesis:
    description: >-
      Setting a ClusterQueue's fairSharing weight to zero could cause a
      division-by-zero in the fair sharing algorithm (known upstream issue
      #2473). The controller should either reject the invalid weight or
      handle it gracefully without panicking. After rollback, normal fair
      sharing resumes with the original weight.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

### kueue-cr-loglevel-corrupt

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operand

Setting the Kueue CR's logLevel to TraceAll forces maximum verbosity on the operand. The kueue operator should detect the config change and reconfigure the controller-manager. This tests whether live config changes cause operand instability or restarts. After rollback, the original log level is restored.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-cr-loglevel-corrupt
spec:
  tier: 5
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: Kueue/cluster
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: kueue-controller-manager
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "60s"
  injection:
    type: CRDMutation
    dangerLevel: high
    parameters:
      apiVersion: "kueue.openshift.io/v1"
      kind: "Kueue"
      name: "cluster"
      path: "spec.logLevel"
      value: "TraceAll"
    ttl: "300s"
  hypothesis:
    description: >-
      Setting the Kueue CR's logLevel to TraceAll forces maximum verbosity
      on the operand. The kueue operator should detect the config change
      and reconfigure the controller-manager. This tests whether live
      config changes cause operand instability or restarts. After rollback,
      the original log level is restored.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
```

</details>

---

### kueue-localqueue-stop-policy

- **Type:** CRDMutation
- **Danger Level:** medium
- **Component:** kueue-operand

Setting a LocalQueue's stopPolicy to HoldAndDrain stops new workload admissions and drains existing ones from this tenant queue. The kueue controller should handle this gracefully without crashing. After rollback (stopPolicy restored to None), admission resumes normally.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-localqueue-stop-policy
spec:
  tier: 2
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: LocalQueue/chaos-test-lq
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    parameters:
      apiVersion: "kueue.x-k8s.io/v1beta1"
      kind: "LocalQueue"
      name: "chaos-test-lq"
      namespace: "chaos-kueue-test"
      path: "spec.stopPolicy"
      value: "HoldAndDrain"
    ttl: "300s"
  hypothesis:
    description: >-
      Setting a LocalQueue's stopPolicy to HoldAndDrain stops new workload
      admissions and drains existing ones from this tenant queue. The kueue
      controller should handle this gracefully without crashing. After
      rollback (stopPolicy restored to None), admission resumes normally.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - chaos-kueue-test
```

</details>

---

### kueue-priority-inversion

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operand

Inverting the priority ordering by setting the high-priority class value to 1 (same as low-priority) tests whether the scheduler caches stale priority values or reads fresh. Subsequent preemption decisions should use the new values. After restoration, normal priority ordering resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-priority-inversion
spec:
  tier: 3
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: WorkloadPriorityClass/chaos-test-high-priority
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    parameters:
      apiVersion: "kueue.x-k8s.io/v1beta1"
      kind: "WorkloadPriorityClass"
      name: "chaos-test-high-priority"
      path: "value"
      value: "1"
    dangerLevel: high
    ttl: "300s"
  hypothesis:
    description: >-
      Inverting the priority ordering by setting the high-priority class
      value to 1 (same as low-priority) tests whether the scheduler
      caches stale priority values or reads fresh. Subsequent preemption
      decisions should use the new values. After restoration, normal
      priority ordering resumes.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

### kueue-resourceflavor-delete-in-use

- **Type:** CRDMutation
- **Danger Level:** high
- **Component:** kueue-operand

Corrupting a ResourceFlavor that is actively referenced by ClusterQueues tests whether the controller handles modified flavor metadata gracefully. The ClusterQueue should continue operating with the existing flavor configuration. After rollback, normal flavor resolution resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-resourceflavor-delete-in-use
spec:
  tier: 3
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: ResourceFlavor/chaos-test-flavor
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: CRDMutation
    dangerLevel: high
    parameters:
      apiVersion: "kueue.x-k8s.io/v1beta1"
      kind: "ResourceFlavor"
      name: "chaos-test-flavor"
      path: "metadata.labels"
      value: "{\"chaos.operatorchaos.io/corrupted\": \"true\"}"
    ttl: "300s"
  hypothesis:
    description: >-
      Corrupting a ResourceFlavor that is actively referenced by ClusterQueues
      tests whether the controller handles modified flavor metadata gracefully.
      The ClusterQueue should continue operating with the existing flavor
      configuration. After rollback, normal flavor resolution resumes.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
```

</details>

---

### kueue-webhook-cert-corrupt

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** kueue-operand

Corrupting the kueue webhook TLS certificate should cause API server webhook calls to fail with TLS errors. New job submissions will be rejected (if failurePolicy is Fail) or bypass validation (if Ignore). The internal cert manager should detect the corruption and regenerate. After rollback or regeneration, webhook functionality resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: kueue-webhook-cert-corrupt
spec:
  tier: 4
  target:
    operator: rh-kueue
    component: kueue-operand
    resource: Secret/kueue-webhook-server-cert
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: openshift-kueue-operator
        namespace: openshift-kueue-operator
        conditionType: Available
    timeout: "30s"
  injection:
    type: ConfigDrift
    dangerLevel: high
    parameters:
      name: kueue-webhook-server-cert
      namespace: openshift-kueue-operator
      key: tls.crt
      value: "corrupted-by-chaos"
      resourceType: Secret
    ttl: "300s"
  hypothesis:
    description: >-
      Corrupting the kueue webhook TLS certificate should cause API server
      webhook calls to fail with TLS errors. New job submissions will be
      rejected (if failurePolicy is Fail) or bypass validation (if Ignore).
      The internal cert manager should detect the corruption and regenerate.
      After rollback or regeneration, webhook functionality resumes.
    recoveryTimeout: 180s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - openshift-kueue-operator
```

</details>


<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
