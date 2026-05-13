# knative-serving Failure Modes

## Coverage

| Injection Type | Danger | Experiment | Description |
|----------------|--------|------------|-------------|
| PodKill | low | activator/pod-kill.yaml | Killing one activator pod should not affect inference traffic since the other re... |
| PodKill | high | activator/all-pods-kill.yaml | Killing both activator pods simultaneously causes a complete traffic blackout fo... |
| NetworkPartition | medium | activator/network-partition.yaml | Isolating activator pods from the network blocks all inference requests routed t... |
| LabelStomping | medium | activator/label-stomping.yaml | Removing the app=activator label from pods disconnects them from the activator S... |
| QuotaExhaustion | medium | activator/quota-exhaustion.yaml | A restrictive ResourceQuota in the knative-serving namespace prevents the activa... |
| RBACRevoke | high | activator/rbac-revoke.yaml | Revoking the activator's ClusterRoleBinding removes its ability to read Services... |
| PodKill | medium | autoscaler/pod-kill.yaml | Killing a single autoscaler pod temporarily pauses scale decisions. The remainin... |
| PodKill | high | autoscaler/all-pods-kill.yaml | Killing both autoscaler pods simultaneously stops all scale-to-zero and scale-up... |
| NetworkPartition | medium | autoscaler/network-partition.yaml | Isolating the autoscaler from the network blocks metric collection from activato... |
| LabelStomping | medium | autoscaler/label-stomping.yaml | Removing the app=autoscaler label disconnects pods from the autoscaler Service. ... |
| QuotaExhaustion | medium | autoscaler/quota-exhaustion.yaml | A restrictive ResourceQuota prevents the autoscaler from restarting. |
| PodKill | medium | autoscaler-hpa/pod-kill.yaml | Killing an autoscaler-hpa pod temporarily pauses HPA-based scaling. The other re... |
| NetworkPartition | medium | autoscaler-hpa/network-partition.yaml | Isolating the HPA autoscaler blocks its ability to receive metrics and update HP... |
| PodKill | low | controller/pod-kill.yaml | Killing one Knative Serving controller pod should not affect existing inference ... |
| PodKill | high | controller/all-pods-kill.yaml | Killing both controller pods stops all Knative Service reconciliation. Existing ... |
| NetworkPartition | medium | controller/network-partition.yaml | Isolating the controller from the network prevents it from reading or updating K... |
| LabelStomping | medium | controller/label-stomping.yaml | Removing the app=controller label from pods. The Deployment controller should re... |
| QuotaExhaustion | medium | controller/quota-exhaustion.yaml | A restrictive ResourceQuota prevents the controller from restarting after failur... |
| RBACRevoke | high | controller/rbac-revoke.yaml | Revoking the controller's admin ClusterRoleBinding removes its ability to manage... |
| PodKill | low | webhook/pod-kill.yaml | Killing one Knative webhook pod should not block Knative Service operations sinc... |
| PodKill | high | webhook/all-pods-kill.yaml | Killing both webhook pods blocks all Knative Service creation and modification. ... |
| NetworkPartition | medium | webhook/network-partition.yaml | Isolating webhook pods makes the webhook Service unreachable. The API server get... |
| ConfigDrift | high | webhook/cert-corrupt.yaml | Corrupting the Knative webhook TLS certificate should cause webhook validation t... |
| LabelStomping | medium | webhook/label-stomping.yaml | Removing the app=webhook label disconnects pods from webhook Service endpoints, ... |
| QuotaExhaustion | medium | webhook/quota-exhaustion.yaml | A restrictive ResourceQuota prevents the webhook from restarting. |
| WebhookDisrupt | high | webhook/webhook-disrupt.yaml | Changing the validating webhook's failurePolicy from Fail to Ignore bypasses all... |
| PodKill | medium | kourier-gateway/pod-kill.yaml | The Kourier gateway is the Envoy-based ingress proxy for all Knative Serving tra... |
| PodKill | high | kourier-gateway/all-pods-kill.yaml | Killing both Kourier gateway pods simultaneously causes a complete inference traf... |
| NetworkPartition | medium | kourier-gateway/network-partition.yaml | Network-isolating the Kourier gateway pods blocks all inference traffic at the i... |
| LabelStomping | medium | kourier-gateway/label-stomping.yaml | Removing the app=3scale-kourier-gateway label disconnects gateway pods from the ... |
| QuotaExhaustion | medium | kourier-gateway/quota-exhaustion.yaml | A restrictive ResourceQuota prevents the kourier gateway from restarting. |
| PodKill | medium | net-kourier-controller/pod-kill.yaml | Killing a net-kourier-controller pod temporarily pauses Envoy route programming.... |
| PodKill | high | net-kourier-controller/all-pods-kill.yaml | Killing both net-kourier-controller pods stops all Envoy route programming. Exis... |
| NetworkPartition | medium | net-kourier-controller/network-partition.yaml | Isolating the net-kourier-controller prevents it from receiving API server event... |
| RBACRevoke | high | net-kourier-controller/rbac-revoke.yaml | Revoking the net-kourier ClusterRoleBinding prevents the controller from reading... |

## Experiment Details

### activator

#### knative-activator-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** activator

The activator is the request-buffering proxy that holds traffic during scale-from-zero. Killing one activator pod should not affect inference traffic since the other replica handles requests. The Deployment controller recreates the pod.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-activator-pod-kill
spec:
  tier: 1
  target:
    operator: knative-serving
    component: activator
    resource: Deployment/activator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: activator
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=activator
    count: 1
    ttl: "120s"
  hypothesis:
    description: >-
      The activator is the request-buffering proxy that holds traffic
      during scale-from-zero. Killing one activator pod should not
      affect inference traffic since the other replica handles requests.
      The Deployment controller recreates the pod.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-activator-all-pods-kill

- **Type:** PodKill
- **Danger Level:** high
- **Component:** activator

Killing both activator pods simultaneously causes a complete traffic blackout for scale-from-zero and request buffering. Active inference services with running pods may still serve directly, but any service in scale-to-zero state becomes unreachable. After Deployment recreates both pods, traffic routing should resume.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-activator-all-pods-kill
spec:
  tier: 4
  target:
    operator: knative-serving
    component: activator
    resource: Deployment/activator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: activator
        namespace: knative-serving
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=activator
    count: 2
    ttl: "120s"
  hypothesis:
    description: >-
      Killing both activator pods simultaneously causes a complete
      traffic blackout for scale-from-zero and request buffering.
      Active inference services with running pods may still serve
      directly, but any service in scale-to-zero state becomes
      unreachable. After Deployment recreates both pods, traffic
      routing should resume.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-activator-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** activator

Isolating activator pods from the network blocks all inference requests routed through the activator. This simulates a network failure between the ingress layer and the activator. The autoscaler loses visibility into request counts, potentially causing erratic scaling decisions. After partition is lifted, traffic routing should resume.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-activator-network-partition
spec:
  tier: 3
  target:
    operator: knative-serving
    component: activator
    resource: Deployment/activator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: activator
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      labelSelector: app=activator
      direction: ingress
    ttl: "60s"
  hypothesis:
    description: >-
      Isolating activator pods from the network blocks all inference
      requests routed through the activator. This simulates a network
      failure between the ingress layer and the activator. The
      autoscaler loses visibility into request counts, potentially
      causing erratic scaling decisions. After partition is lifted,
      traffic routing should resume.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-activator-label-stomping

- **Type:** LabelStomping
- **Danger Level:** medium
- **Component:** activator

Removing the app=activator label from pods disconnects them from the activator Service endpoints. Traffic can no longer reach the activator. The Deployment controller should recreate pods with correct labels or the existing pods should be re-labeled by the operator.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-activator-label-stomping
spec:
  tier: 3
  target:
    operator: knative-serving
    component: activator
    resource: Deployment/activator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: activator
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: LabelStomping
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: activator
      labelKey: app
      action: overwrite
    ttl: "120s"
  hypothesis:
    description: >-
      Removing the app=activator label from pods disconnects them from the
      activator Service endpoints. Traffic can no longer reach the activator.
      The Deployment controller should recreate pods with correct labels or
      the existing pods should be re-labeled by the operator.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-activator-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** medium
- **Component:** activator

A restrictive ResourceQuota in the knative-serving namespace prevents the activator from restarting after a failure. If the pods are evicted or crash, new pods cannot be scheduled until the quota is relaxed.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-activator-quota-exhaustion
spec:
  tier: 3
  target:
    operator: knative-serving
    component: activator
    resource: Deployment/activator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: activator
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: QuotaExhaustion
    parameters:
      quotaName: chaos-quota-activator
      pods: "0"
      cpu: "0"
      memory: "0"
    ttl: "60s"
  hypothesis:
    description: >-
      A restrictive ResourceQuota in the knative-serving namespace prevents
      the activator from restarting after a failure. If the pods are evicted
      or crash, new pods cannot be scheduled until the quota is relaxed.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-activator-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** activator

Revoking the activator's ClusterRoleBinding removes its ability to read Services, Endpoints, and other resources. The activator can't proxy traffic correctly without these permissions. After RBAC is restored, the activator should resume normal operation without restart.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-activator-rbac-revoke
spec:
  tier: 4
  target:
    operator: knative-serving
    component: activator
    resource: Deployment/activator
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: activator
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: RBACRevoke
    dangerLevel: high
    parameters:
      bindingName: knative-serving-activator-cluster
      bindingType: ClusterRoleBinding
    ttl: "120s"
  hypothesis:
    description: >-
      Revoking the activator's ClusterRoleBinding removes its ability to
      read Services, Endpoints, and other resources. The activator can't
      proxy traffic correctly without these permissions. After RBAC is
      restored, the activator should resume normal operation without restart.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
```

</details>

---

### autoscaler

#### knative-autoscaler-pod-kill

- **Type:** PodKill
- **Danger Level:** medium
- **Component:** autoscaler

Killing a single autoscaler pod temporarily pauses scale decisions. The remaining replica should take over via leader election. Workloads continue running but may not scale correctly during the brief outage.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-autoscaler-pod-kill
spec:
  tier: 3
  target:
    operator: knative-serving
    component: autoscaler
    resource: Deployment/autoscaler
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: autoscaler
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=autoscaler
    count: 1
    ttl: "120s"
  hypothesis:
    description: >-
      Killing a single autoscaler pod temporarily pauses scale decisions.
      The remaining replica should take over via leader election. Workloads
      continue running but may not scale correctly during the brief outage.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-autoscaler-all-pods-kill

- **Type:** PodKill
- **Danger Level:** high
- **Component:** autoscaler

Killing both autoscaler pods simultaneously stops all scale-to-zero and scale-up decisions. Running inference pods continue serving but no new scaling events are processed. After recovery, the autoscaler reads current metrics and resumes scaling.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-autoscaler-all-pods-kill
spec:
  tier: 4
  target:
    operator: knative-serving
    component: autoscaler
    resource: Deployment/autoscaler
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: autoscaler
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=autoscaler
    count: 2
    ttl: "120s"
  hypothesis:
    description: >-
      Killing both autoscaler pods simultaneously stops all scale-to-zero
      and scale-up decisions. Running inference pods continue serving but
      no new scaling events are processed. After recovery, the autoscaler
      reads current metrics and resumes scaling.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-autoscaler-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** autoscaler

Isolating the autoscaler from the network blocks metric collection from activator pods and prevents scale decisions from being communicated. Running pods continue but autoscaling stalls. This is particularly interesting because the activator's health check depends on a WebSocket to the autoscaler.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-autoscaler-network-partition
spec:
  tier: 3
  target:
    operator: knative-serving
    component: autoscaler
    resource: Deployment/autoscaler
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: autoscaler
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      direction: ingress
      labelSelector: app=autoscaler
    ttl: "120s"
  hypothesis:
    description: >-
      Isolating the autoscaler from the network blocks metric collection
      from activator pods and prevents scale decisions from being communicated.
      Running pods continue but autoscaling stalls. This is particularly
      interesting because the activator's health check depends on a WebSocket
      to the autoscaler.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-autoscaler-label-stomping

- **Type:** LabelStomping
- **Danger Level:** medium
- **Component:** autoscaler

Removing the app=autoscaler label disconnects pods from the autoscaler Service. The Deployment controller should recreate pods with the correct labels.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-autoscaler-label-stomping
spec:
  tier: 3
  target:
    operator: knative-serving
    component: autoscaler
    resource: Deployment/autoscaler
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: autoscaler
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: LabelStomping
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: autoscaler
      labelKey: app
      action: overwrite
    ttl: "120s"
  hypothesis:
    description: >-
      Removing the app=autoscaler label disconnects pods from the autoscaler
      Service. The Deployment controller should recreate pods with the correct
      labels.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-autoscaler-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** medium
- **Component:** autoscaler

A restrictive ResourceQuota prevents the autoscaler from restarting.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-autoscaler-quota-exhaustion
spec:
  tier: 3
  target:
    operator: knative-serving
    component: autoscaler
    resource: Deployment/autoscaler
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: autoscaler
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: QuotaExhaustion
    parameters:
      quotaName: chaos-quota-autoscaler
      pods: "0"
      cpu: "0"
      memory: "0"
    ttl: "60s"
  hypothesis:
    description: >-
      A restrictive ResourceQuota prevents the autoscaler from restarting.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

### autoscaler-hpa

#### knative-autoscaler-hpa-pod-kill

- **Type:** PodKill
- **Danger Level:** medium
- **Component:** autoscaler-hpa

Killing an autoscaler-hpa pod temporarily pauses HPA-based scaling. The other replica takes over via leader election.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-autoscaler-hpa-pod-kill
spec:
  tier: 3
  target:
    operator: knative-serving
    component: autoscaler-hpa
    resource: Deployment/autoscaler-hpa
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: autoscaler-hpa
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=autoscaler-hpa
    count: 1
    ttl: "120s"
  hypothesis:
    description: >-
      Killing an autoscaler-hpa pod temporarily pauses HPA-based scaling.
      The other replica takes over via leader election.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-autoscaler-hpa-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** autoscaler-hpa

Isolating the HPA autoscaler blocks its ability to receive metrics and update HPA resources. Existing HPA targets maintain their current replica count but cannot adjust.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-autoscaler-hpa-network-partition
spec:
  tier: 3
  target:
    operator: knative-serving
    component: autoscaler-hpa
    resource: Deployment/autoscaler-hpa
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: autoscaler-hpa
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      direction: ingress
      labelSelector: app=autoscaler-hpa
    ttl: "120s"
  hypothesis:
    description: >-
      Isolating the HPA autoscaler blocks its ability to receive metrics
      and update HPA resources. Existing HPA targets maintain their current
      replica count but cannot adjust.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

### controller

#### knative-controller-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** controller

Killing one Knative Serving controller pod should not affect existing inference services since the other replica takes over leader election. New Knative Service creation may briefly stall during leader transition.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-controller-pod-kill
spec:
  tier: 1
  target:
    operator: knative-serving
    component: controller
    resource: Deployment/controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: controller
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=controller
    count: 1
    ttl: "120s"
  hypothesis:
    description: >-
      Killing one Knative Serving controller pod should not affect
      existing inference services since the other replica takes over
      leader election. New Knative Service creation may briefly stall
      during leader transition.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-controller-all-pods-kill

- **Type:** PodKill
- **Danger Level:** high
- **Component:** controller

Killing both controller pods stops all Knative Service reconciliation. Existing services continue running but new deployments, updates, and route changes are not processed. After recovery, leader election completes and reconciliation resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-controller-all-pods-kill
spec:
  tier: 4
  target:
    operator: knative-serving
    component: controller
    resource: Deployment/controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: controller
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=controller
    count: 2
    ttl: "120s"
  hypothesis:
    description: >-
      Killing both controller pods stops all Knative Service reconciliation.
      Existing services continue running but new deployments, updates, and
      route changes are not processed. After recovery, leader election
      completes and reconciliation resumes.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-controller-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** controller

Isolating the controller from the network prevents it from reading or updating Knative resources. Reconciliation stalls. Existing services continue running. After the partition lifts, the controller should resume reconciliation.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-controller-network-partition
spec:
  tier: 3
  target:
    operator: knative-serving
    component: controller
    resource: Deployment/controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: controller
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      direction: ingress
      labelSelector: app=controller
    ttl: "120s"
  hypothesis:
    description: >-
      Isolating the controller from the network prevents it from reading
      or updating Knative resources. Reconciliation stalls. Existing services
      continue running. After the partition lifts, the controller should
      resume reconciliation.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-controller-label-stomping

- **Type:** LabelStomping
- **Danger Level:** medium
- **Component:** controller

Removing the app=controller label from pods. The Deployment controller should recreate pods with correct labels.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-controller-label-stomping
spec:
  tier: 3
  target:
    operator: knative-serving
    component: controller
    resource: Deployment/controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: controller
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: LabelStomping
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: controller
      labelKey: app
      action: overwrite
    ttl: "120s"
  hypothesis:
    description: >-
      Removing the app=controller label from pods. The Deployment controller
      should recreate pods with correct labels.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-controller-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** medium
- **Component:** controller

A restrictive ResourceQuota prevents the controller from restarting after failure.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-controller-quota-exhaustion
spec:
  tier: 3
  target:
    operator: knative-serving
    component: controller
    resource: Deployment/controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: controller
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: QuotaExhaustion
    parameters:
      quotaName: chaos-quota-controller
      pods: "0"
      cpu: "0"
      memory: "0"
    ttl: "60s"
  hypothesis:
    description: >-
      A restrictive ResourceQuota prevents the controller from restarting
      after failure.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-controller-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** controller

Revoking the controller's admin ClusterRoleBinding removes its ability to manage Knative resources (Services, Routes, Configurations, Revisions). All reconciliation fails with authorization errors. After RBAC is restored, reconciliation should resume.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-controller-rbac-revoke
spec:
  tier: 4
  target:
    operator: knative-serving
    component: controller
    resource: Deployment/controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: controller
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: RBACRevoke
    dangerLevel: high
    parameters:
      bindingName: knative-serving-controller-admin
      bindingType: ClusterRoleBinding
    ttl: "120s"
  hypothesis:
    description: >-
      Revoking the controller's admin ClusterRoleBinding removes its ability
      to manage Knative resources (Services, Routes, Configurations, Revisions).
      All reconciliation fails with authorization errors. After RBAC is
      restored, reconciliation should resume.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
```

</details>

---

### webhook

#### knative-webhook-pod-kill

- **Type:** PodKill
- **Danger Level:** low
- **Component:** webhook

Killing one Knative webhook pod should not block Knative Service operations since the other replica handles validation/mutation. Existing running services are unaffected.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-webhook-pod-kill
spec:
  tier: 1
  target:
    operator: knative-serving
    component: webhook
    resource: Deployment/webhook
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: webhook
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=webhook
    count: 1
    ttl: "120s"
  hypothesis:
    description: >-
      Killing one Knative webhook pod should not block Knative Service
      operations since the other replica handles validation/mutation.
      Existing running services are unaffected.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-webhook-all-pods-kill

- **Type:** PodKill
- **Danger Level:** high
- **Component:** webhook

Killing both webhook pods blocks all Knative Service creation and modification. The API server cannot validate or mutate Knative resources. Existing services are unaffected but cannot be updated.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-webhook-all-pods-kill
spec:
  tier: 5
  target:
    operator: knative-serving
    component: webhook
    resource: Deployment/webhook
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: webhook
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=webhook
    count: 2
    ttl: "120s"
  hypothesis:
    description: >-
      Killing both webhook pods blocks all Knative Service creation and
      modification. The API server cannot validate or mutate Knative resources.
      Existing services are unaffected but cannot be updated.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-webhook-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** webhook

Isolating webhook pods makes the webhook Service unreachable. The API server gets timeout errors when validating Knative resources. With failurePolicy=Fail (default), all Knative operations are blocked.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-webhook-network-partition
spec:
  tier: 3
  target:
    operator: knative-serving
    component: webhook
    resource: Deployment/webhook
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: webhook
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      direction: ingress
      labelSelector: app=webhook
    ttl: "120s"
  hypothesis:
    description: >-
      Isolating webhook pods makes the webhook Service unreachable. The API
      server gets timeout errors when validating Knative resources. With
      failurePolicy=Fail (default), all Knative operations are blocked.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-webhook-cert-corrupt

- **Type:** ConfigDrift
- **Danger Level:** high
- **Component:** webhook

Corrupting the Knative webhook TLS certificate should cause webhook validation to fail with TLS errors. This blocks creation and modification of all Knative Service resources. Existing services continue running but cannot be updated. The webhook or cert-manager should detect and regenerate the certificate.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-webhook-cert-corrupt
spec:
  tier: 3
  target:
    operator: knative-serving
    component: webhook
    resource: Secret/webhook-certs
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: webhook
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: ConfigDrift
    dangerLevel: high
    parameters:
      name: webhook-certs
      key: server-cert.pem
      value: "Y2hhb3MtY29ycnVwdGVk"
      resourceType: Secret
    ttl: "60s"
  hypothesis:
    description: >-
      Corrupting the Knative webhook TLS certificate should cause webhook
      validation to fail with TLS errors. This blocks creation and
      modification of all Knative Service resources. Existing services
      continue running but cannot be updated. The webhook or cert-manager
      should detect and regenerate the certificate.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-webhook-label-stomping

- **Type:** LabelStomping
- **Danger Level:** medium
- **Component:** webhook

Removing the app=webhook label disconnects pods from webhook Service endpoints, making the webhook unreachable for API server calls.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-webhook-label-stomping
spec:
  tier: 3
  target:
    operator: knative-serving
    component: webhook
    resource: Deployment/webhook
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: webhook
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: LabelStomping
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: webhook
      labelKey: app
      action: overwrite
    ttl: "120s"
  hypothesis:
    description: >-
      Removing the app=webhook label disconnects pods from webhook Service
      endpoints, making the webhook unreachable for API server calls.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-webhook-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** medium
- **Component:** webhook

A restrictive ResourceQuota prevents the webhook from restarting.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-webhook-quota-exhaustion
spec:
  tier: 3
  target:
    operator: knative-serving
    component: webhook
    resource: Deployment/webhook
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: webhook
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: QuotaExhaustion
    parameters:
      quotaName: chaos-quota-webhook
      pods: "0"
      cpu: "0"
      memory: "0"
    ttl: "60s"
  hypothesis:
    description: >-
      A restrictive ResourceQuota prevents the webhook from restarting.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving
```

</details>

---

#### knative-webhook-webhook-disrupt

- **Type:** WebhookDisrupt
- **Danger Level:** high
- **Component:** webhook

Changing the validating webhook's failurePolicy from Fail to Ignore bypasses all Knative resource validation. Invalid resources can be created. After restoration, validation resumes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-webhook-webhook-disrupt
spec:
  tier: 4
  target:
    operator: knative-serving
    component: webhook
    resource: Deployment/webhook
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: webhook
        namespace: knative-serving
        conditionType: Available
    timeout: "30s"
  injection:
    type: WebhookDisrupt
    dangerLevel: high
    parameters:
      webhookName: validation.webhook.serving.knative.dev
      action: setFailurePolicy
      value: Ignore
    ttl: "60s"
  hypothesis:
    description: >-
      Changing the validating webhook's failurePolicy from Fail to Ignore
      bypasses all Knative resource validation. Invalid resources can be
      created. After restoration, validation resumes.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
```

</details>

---

### kourier-gateway

#### knative-kourier-gateway-pod-kill

- **Type:** PodKill
- **Danger Level:** medium
- **Component:** kourier-gateway

The Kourier gateway is the Envoy-based ingress proxy for all Knative Serving traffic. Killing one gateway pod should shift traffic to the other replica. Brief request failures may occur during the transition. After pod restart, traffic routing resumes normally.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-kourier-gateway-pod-kill
spec:
  tier: 2
  target:
    operator: knative-serving
    component: kourier-gateway
    resource: Deployment/3scale-kourier-gateway
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: 3scale-kourier-gateway
        namespace: knative-serving-ingress
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=3scale-kourier-gateway
    count: 1
    ttl: "120s"
  hypothesis:
    description: >-
      The Kourier gateway is the Envoy-based ingress proxy for all
      Knative Serving traffic. Killing one gateway pod should shift
      traffic to the other replica. Brief request failures may occur
      during the transition. After pod restart, traffic routing
      resumes normally.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - knative-serving-ingress
```

</details>

---

#### knative-kourier-all-pods-kill

- **Type:** PodKill
- **Danger Level:** high
- **Component:** kourier-gateway

Killing both Kourier gateway pods simultaneously causes a complete inference traffic blackout. No external requests can reach any Knative Service. This is the most impactful single fault for inference availability. The Deployment controller should recreate both pods and Envoy should reload its config from the control plane. During the outage, all inference requests fail.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-kourier-all-pods-kill
spec:
  tier: 5
  target:
    operator: knative-serving
    component: kourier-gateway
    resource: Deployment/3scale-kourier-gateway
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: 3scale-kourier-gateway
        namespace: knative-serving-ingress
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=3scale-kourier-gateway
    count: 2
    ttl: "120s"
  hypothesis:
    description: >-
      Killing both Kourier gateway pods simultaneously causes a
      complete inference traffic blackout. No external requests can
      reach any Knative Service. This is the most impactful single
      fault for inference availability. The Deployment controller
      should recreate both pods and Envoy should reload its config
      from the control plane. During the outage, all inference
      requests fail.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving-ingress
```

</details>

---

#### knative-kourier-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** kourier-gateway

Network-isolating the Kourier gateway pods blocks all inference traffic at the ingress layer. Unlike pod kill, the pods remain running but cannot accept connections. This simulates a network failure between the OpenShift router and the Knative ingress. After the partition lifts, Envoy should resume serving traffic without needing a restart.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-kourier-network-partition
spec:
  tier: 3
  target:
    operator: knative-serving
    component: kourier-gateway
    resource: Deployment/3scale-kourier-gateway
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: 3scale-kourier-gateway
        namespace: knative-serving-ingress
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      labelSelector: app=3scale-kourier-gateway
      direction: ingress
    ttl: "60s"
  hypothesis:
    description: >-
      Network-isolating the Kourier gateway pods blocks all inference
      traffic at the ingress layer. Unlike pod kill, the pods remain
      running but cannot accept connections. This simulates a network
      failure between the OpenShift router and the Knative ingress.
      After the partition lifts, Envoy should resume serving traffic
      without needing a restart.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving-ingress
```

</details>

---

#### knative-kourier-gateway-label-stomping

- **Type:** LabelStomping
- **Danger Level:** medium
- **Component:** kourier-gateway

Removing the app=3scale-kourier-gateway label disconnects gateway pods from the gateway Service. External traffic cannot reach any Knative Service.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-kourier-gateway-label-stomping
spec:
  tier: 3
  target:
    operator: knative-serving
    component: kourier-gateway
    resource: Deployment/3scale-kourier-gateway
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: 3scale-kourier-gateway
        namespace: knative-serving-ingress
        conditionType: Available
    timeout: "30s"
  injection:
    type: LabelStomping
    parameters:
      apiVersion: apps/v1
      kind: Deployment
      name: 3scale-kourier-gateway
      labelKey: app
      action: overwrite
    ttl: "120s"
  hypothesis:
    description: >-
      Removing the app=3scale-kourier-gateway label disconnects gateway pods
      from the gateway Service. External traffic cannot reach any Knative
      Service.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving-ingress
```

</details>

---

#### knative-kourier-gateway-quota-exhaustion

- **Type:** QuotaExhaustion
- **Danger Level:** medium
- **Component:** kourier-gateway

A restrictive ResourceQuota prevents the kourier gateway from restarting.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-kourier-gateway-quota-exhaustion
spec:
  tier: 3
  target:
    operator: knative-serving
    component: kourier-gateway
    resource: Deployment/3scale-kourier-gateway
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: 3scale-kourier-gateway
        namespace: knative-serving-ingress
        conditionType: Available
    timeout: "30s"
  injection:
    type: QuotaExhaustion
    parameters:
      quotaName: chaos-quota-kourier-gateway
      pods: "0"
      cpu: "0"
      memory: "0"
    ttl: "60s"
  hypothesis:
    description: >-
      A restrictive ResourceQuota prevents the kourier gateway from restarting.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving-ingress
```

</details>

---

### net-kourier-controller

#### knative-net-kourier-controller-pod-kill

- **Type:** PodKill
- **Danger Level:** medium
- **Component:** net-kourier-controller

Killing a net-kourier-controller pod temporarily pauses Envoy route programming. Existing routes continue working but new Knative Service routes are not programmed. The other replica takes over.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-net-kourier-controller-pod-kill
spec:
  tier: 3
  target:
    operator: knative-serving
    component: net-kourier-controller
    resource: Deployment/net-kourier-controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: net-kourier-controller
        namespace: knative-serving-ingress
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=net-kourier-controller
    count: 1
    ttl: "120s"
  hypothesis:
    description: >-
      Killing a net-kourier-controller pod temporarily pauses Envoy route
      programming. Existing routes continue working but new Knative Service
      routes are not programmed. The other replica takes over.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 1
    allowDangerous: true
    allowedNamespaces:
      - knative-serving-ingress
```

</details>

---

#### knative-net-kourier-controller-all-pods-kill

- **Type:** PodKill
- **Danger Level:** high
- **Component:** net-kourier-controller

Killing both net-kourier-controller pods stops all Envoy route programming. Existing routes continue serving but no new routes are added. After recovery, the controller re-syncs and programs missing routes.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-net-kourier-controller-all-pods-kill
spec:
  tier: 4
  target:
    operator: knative-serving
    component: net-kourier-controller
    resource: Deployment/net-kourier-controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: net-kourier-controller
        namespace: knative-serving-ingress
        conditionType: Available
    timeout: "30s"
  injection:
    type: PodKill
    parameters:
      labelSelector: app=net-kourier-controller
    count: 2
    ttl: "120s"
  hypothesis:
    description: >-
      Killing both net-kourier-controller pods stops all Envoy route
      programming. Existing routes continue serving but no new routes are
      added. After recovery, the controller re-syncs and programs missing
      routes.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving-ingress
```

</details>

---

#### knative-net-kourier-controller-network-partition

- **Type:** NetworkPartition
- **Danger Level:** medium
- **Component:** net-kourier-controller

Isolating the net-kourier-controller prevents it from receiving API server events and updating Envoy configuration. Existing routes remain but new Knative Services cannot be exposed.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-net-kourier-controller-network-partition
spec:
  tier: 3
  target:
    operator: knative-serving
    component: net-kourier-controller
    resource: Deployment/net-kourier-controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: net-kourier-controller
        namespace: knative-serving-ingress
        conditionType: Available
    timeout: "30s"
  injection:
    type: NetworkPartition
    parameters:
      direction: ingress
      labelSelector: app=net-kourier-controller
    ttl: "120s"
  hypothesis:
    description: >-
      Isolating the net-kourier-controller prevents it from receiving API
      server events and updating Envoy configuration. Existing routes remain
      but new Knative Services cannot be exposed.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
    allowedNamespaces:
      - knative-serving-ingress
```

</details>

---

#### knative-net-kourier-controller-rbac-revoke

- **Type:** RBACRevoke
- **Danger Level:** high
- **Component:** net-kourier-controller

Revoking the net-kourier ClusterRoleBinding prevents the controller from reading Ingress resources and programming Envoy. New routes cannot be created. After RBAC is restored, the controller should re-sync.

<details>
<summary>Experiment YAML</summary>

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: knative-net-kourier-controller-rbac-revoke
spec:
  tier: 4
  target:
    operator: knative-serving
    component: net-kourier-controller
    resource: Deployment/net-kourier-controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: net-kourier-controller
        namespace: knative-serving-ingress
        conditionType: Available
    timeout: "30s"
  injection:
    type: RBACRevoke
    dangerLevel: high
    parameters:
      bindingName: net-kourier
      bindingType: ClusterRoleBinding
    ttl: "120s"
  hypothesis:
    description: >-
      Revoking the net-kourier ClusterRoleBinding prevents the controller
      from reading Ingress resources and programming Envoy. New routes cannot
      be created. After RBAC is restored, the controller should re-sync.
    recoveryTimeout: 120s
  blastRadius:
    maxPodsAffected: 2
    allowDangerous: true
```

</details>


<!-- custom-start: known-issues -->
<!-- custom-end: known-issues -->
