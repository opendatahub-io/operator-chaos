# Use Cases

Operator Chaos is a generic framework that works with any Kubernetes operator. The following operators have been validated with chaos experiments.

## Validated Operators

| Operator | Components | Experiments | Failure Modes | Status |
|----------|-----------|-------------|---------------|--------|
| [RHOAI](dashboard/index.md) | 14 | 65+ | 20 | Actively tested |
| [ODH](dashboard/index.md) | 12 | 65+ | 20 | Same components as RHOAI, different namespace |
| [OpenShift Service Mesh](service-mesh/index.md) | 2 | 22 | 14 | All Resilient |
| [Red Hat Build of Kueue](kueue/index.md) | 3 | 30 | 11 | 29/30 Resilient |
| [OpenShift Serverless](knative-serving/index.md) | 7 | 35+ | 10 | 34/35 Resilient |
| [cert-manager](cert-manager/index.md) | 3 | 14+ | 10 | All Resilient |
| [Strimzi (AMQ Streams)](strimzi/index.md) | 1 | 8 | 8 | 7/8 Resilient, 1 Degraded |
| [Spark Operator](spark-operator/index.md) | 2 | 12 | 9 | 9/12 Resilient, 2 Degraded, 1 Failed |
| [ArgoCD (OpenShift GitOps)](argocd/index.md) | 2 | 16 | 8 | All Resilient |
| [Tekton (OpenShift Pipelines)](tekton/index.md) | 2 | 14 | 8 | All Resilient |
| [Prometheus Operator](prometheus-operator/index.md) | 1 | 6 | 6 | All Resilient |

## Coverage Matrix

### RHOAI / ODH Components: Core Failure Modes

Abbreviations: PK (PodKill), NP (NetworkPartition), QE (QuotaExhaustion), ND (NamespaceDeletion), CD (ConfigDrift), CM (CRDMutation), LS (LabelStomping), CF (ClientFault), RR (RBACRevoke), WD (WebhookDisrupt), WL (WebhookLatency), FB (FinalizerBlock), OR (OwnerRefOrphan)

| Component | PK | NP | QE | ND | CD | CM | LS | CF | RR | WD | WL | FB | OR |
|-----------|----|----|----|----|----|----|----|----|----|----|----|----|------|
| [dashboard](dashboard/index.md) | Y | Y | Y | - | Y | Y | - | - | Y | Y | - | - | - |
| [data-science-pipelines](data-science-pipelines/index.md) | Y | Y | - | - | Y | - | - | - | Y | Y | - | Y | - |
| [feast](feast/index.md) | Y | Y | - | - | - | - | - | - | Y | Y | - | Y | - |
| [kserve](kserve/index.md) | Y | Y | - | - | Y | Y | - | - | - | Y | - | - | Y |
| [llamastack](llamastack/index.md) | Y | Y | - | - | Y | - | - | - | Y | - | - | - | - |
| [model-registry](model-registry/index.md) | Y | Y | - | - | - | Y | - | - | Y | Y | - | Y | - |
| [odh-model-controller](odh-model-controller/index.md) | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y |
| [opendatahub-operator](opendatahub-operator/index.md) | Y | Y | - | - | - | - | - | - | Y | Y | - | Y | - |
| [ray](ray/index.md) | Y | Y | - | - | - | - | - | - | Y | Y | - | Y | - |
| [training-operator](training-operator/index.md) | Y | Y | - | - | - | - | - | - | Y | Y | - | Y | - |
| [trustyai](trustyai/index.md) | Y | Y | - | - | - | - | - | - | Y | Y | - | Y | - |
| [workbenches](workbenches/index.md) | Y | Y | - | - | - | - | - | - | Y | Y | - | - | - |
| [codeflare](codeflare/index.md) | Y | Y | - | - | Y | - | - | - | Y | Y | - | Y | - |
| [modelmesh](modelmesh/index.md) | Y | Y | - | - | Y | - | - | - | Y | Y | - | Y | - |

### RHOAI / ODH Components: Extended Failure Modes

Abbreviations: OL (OLM Lifecycle), SD (SecretDeletion), SZ (DeploymentScaleZero), LE (LeaderElectionDisrupt), CL (CrashLoopInject), IC (ImageCorrupt), RD (ResourceDeletion), PB (PDBBlock)

| Component | OL | SD | SZ | LE | CL | IC | RD | PB |
|-----------|----|----|----|----|----|----|----|----|
| [dashboard](dashboard/index.md) | - | - | - | - | - | - | - | - |
| [data-science-pipelines](data-science-pipelines/index.md) | - | - | - | - | - | - | - | - |
| [feast](feast/index.md) | - | - | - | - | - | - | - | - |
| [kserve](kserve/index.md) | - | - | - | - | Y | Y | Y | Y |
| [llamastack](llamastack/index.md) | - | - | - | - | - | - | - | - |
| [model-registry](model-registry/index.md) | - | - | - | - | - | - | - | - |
| [odh-model-controller](odh-model-controller/index.md) | - | - | - | - | Y | Y | Y | Y |
| [opendatahub-operator](opendatahub-operator/index.md) | Y | - | - | - | - | - | - | - |
| [ray](ray/index.md) | - | - | - | - | - | - | - | - |
| [training-operator](training-operator/index.md) | - | - | - | - | - | - | - | - |
| [trustyai](trustyai/index.md) | - | - | - | - | - | - | - | - |
| [workbenches](workbenches/index.md) | - | - | - | - | - | - | - | - |
| [codeflare](codeflare/index.md) | - | - | - | - | - | - | - | - |
| [modelmesh](modelmesh/index.md) | - | - | - | - | - | - | - | - |

### OpenShift Service Mesh: Core Failure Modes

| Component | PK | NP | QE | CD | CM | LS | RR | WD | WL | FB | OR |
|-----------|----|----|----|----|----|----|----|----|----|----|------|
| servicemesh-operator3 | Y | Y | Y | - | Y | Y | Y | - | - | - | - |
| istiod | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y |

### OpenShift Service Mesh: Extended Failure Modes

| Component | OL | CL | IC | RD | PB |
|-----------|----|----|----|----|------|
| servicemesh-operator3 | Y | - | - | - | - |
| istiod | - | Y | Y | Y | Y |

### Red Hat Build of Kueue: Core Failure Modes

| Component | PK | NP | QE | CD | CM | LS | RR | WD | FB | OR |
|-----------|----|----|----|----|----|----|----|----|----|----|------|
| kueue (legacy) | Y | Y | - | - | - | - | Y | Y | Y | - |
| kueue-operator | Y | Y | Y | - | Y | Y | Y | - | - | Y |
| kueue-operand | Y | - | - | Y | Y | - | - | - | - | - |

### Red Hat Build of Kueue: Extended Failure Modes

| Component | SZ | LE |
|-----------|----|----|
| kueue (legacy) | - | - |
| kueue-operator | Y | Y |
| kueue-operand | Y | - |

### OpenShift Serverless (Knative Serving): Core Failure Modes

| Component | PK | NP | QE | LS | RR | WD | CD |
|-----------|----|----|----|----|----|----|------|
| activator | Y | Y | Y | Y | Y | - | - |
| autoscaler | Y | Y | Y | Y | - | - | - |
| autoscaler-hpa | Y | Y | - | - | - | - | - |
| controller | Y | Y | Y | Y | Y | - | - |
| webhook | Y | Y | Y | Y | - | Y | Y |
| kourier-gateway | Y | Y | Y | Y | - | - | - |
| net-kourier-controller | Y | Y | - | - | Y | - | - |

### cert-manager Operator: Core Failure Modes

| Component | PK | NP | QE | LS | RR | CD |
|-----------|----|----|----|----|----|----|------|
| controller | Y | Y | Y | Y | Y | - |
| webhook | Y | Y | Y | Y | - | Y |
| cainjector | Y | Y | Y | Y | - | - |
