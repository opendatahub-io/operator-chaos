# Use Cases

Operator Chaos is a generic framework that works with any Kubernetes operator. The following operators have been validated with chaos experiments.

## Validated Operators

| Operator | Components | Experiments | Failure Modes | Status |
|----------|-----------|-------------|---------------|--------|
| [RHOAI / ODH](dashboard/index.md) | 14 | 200+ | 20 | Actively tested |
| [OpenShift Service Mesh](service-mesh/index.md) | 2 | 22 | 14 | All Resilient |
| [Red Hat Build of Kueue](kueue/index.md) | 3 | 38 | 11 | All Resilient |
| [OpenShift Serverless](knative-serving/index.md) | 7 | 39 | 10 | 38/39 Resilient |
| [cert-manager](cert-manager/index.md) | 3 | 18 | 10 | All Resilient |
| [Strimzi (AMQ Streams)](strimzi/index.md) | 1 | 8 | 8 | 7/8 Resilient |
| [Spark Operator](spark-operator/index.md) | 2 | 12 | 9 | 9/12 Resilient |
| [ArgoCD (OpenShift GitOps)](argocd/index.md) | 2 | 16 | 8 | All Resilient |
| [Tekton (OpenShift Pipelines)](tekton/index.md) | 2 | 14 | 8 | All Resilient |
| [Prometheus Operator](prometheus-operator/index.md) | 1 | 6 | 6 | All Resilient |

## Coverage Matrix

### RHOAI / ODH Components: Core Failure Modes

Abbreviations: PK (PodKill), NP (NetworkPartition), QE (QuotaExhaustion), ND (NamespaceDeletion), CD (ConfigDrift), CM (CRDMutation), LS (LabelStomping), CF (ClientFault), RR (RBACRevoke), WD (WebhookDisrupt), WL (WebhookLatency), FB (FinalizerBlock), OR (OwnerRefOrphan)

| Component | PK | NP | QE | ND | CD | CM | LS | CF | RR | WD | WL | FB | OR |
|----|----|----|----|----|----|----|----|----|----|----|----|----|----|
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
| [dashboard](dashboard/index.md) | - | - | Y | - | Y | Y | Y | Y |
| [data-science-pipelines](data-science-pipelines/index.md) | - | - | Y | Y | Y | Y | Y | Y |
| [feast](feast/index.md) | - | - | Y | Y | Y | Y | Y | Y |
| [kserve](kserve/index.md) | - | - | - | - | Y | Y | Y | Y |
| [llamastack](llamastack/index.md) | - | - | Y | Y | Y | Y | Y | Y |
| [model-registry](model-registry/index.md) | - | - | Y | Y | Y | Y | Y | Y |
| [odh-model-controller](odh-model-controller/index.md) | - | - | - | - | Y | Y | Y | Y |
| [opendatahub-operator](opendatahub-operator/index.md) | Y | - | - | - | - | - | - | - |
| [ray](ray/index.md) | - | - | Y | Y | Y | Y | Y | Y |
| [training-operator](training-operator/index.md) | - | - | Y | - | Y | Y | Y | Y |
| [trustyai](trustyai/index.md) | - | - | Y | Y | Y | Y | Y | Y |
| [workbenches](workbenches/index.md) | - | - | Y | - | Y | Y | Y | Y |
| [codeflare](codeflare/index.md) | - | - | - | - | - | - | - | - |
| [modelmesh](modelmesh/index.md) | - | - | - | - | - | - | - | - |

### OpenShift Service Mesh: Core Failure Modes

| Component | PK | NP | QE | CD | CM | LS | RR | WD | WL | FB | OR |
|----|----|----|----|----|----|----|----|----|----|----|----|
| servicemesh-operator3 | Y | Y | Y | - | Y | Y | Y | - | - | - | - |
| istiod | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y |

### OpenShift Service Mesh: Extended Failure Modes

| Component | OL | CL | IC | RD | PB |
|----|----|----|----|----|----|
| servicemesh-operator3 | Y | - | - | - | - |
| istiod | - | Y | Y | Y | Y |

### Red Hat Build of Kueue: Core Failure Modes

| Component | PK | NP | QE | CD | CM | LS | RR | WD | FB | OR |
|-----------|----|----|----|----|----|----|----|----|----|----|
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
|----|----|----|----|----|----|----|----|
| activator | Y | Y | Y | Y | Y | - | - |
| autoscaler | Y | Y | Y | Y | - | - | - |
| autoscaler-hpa | Y | Y | - | - | - | - | - |
| controller | Y | Y | Y | Y | Y | - | - |
| webhook | Y | Y | Y | Y | - | Y | Y |
| kourier-gateway | Y | Y | Y | Y | - | - | - |
| net-kourier-controller | Y | Y | - | - | Y | - | - |

### OpenShift Serverless (Knative Serving): Extended Failure Modes

| Component | CL | IC | RD | PB |
|-----------|----|----|----|----|
| activator | - | - | - | - |
| autoscaler | - | - | - | - |
| autoscaler-hpa | - | - | - | - |
| controller | Y | Y | Y | Y |
| webhook | - | - | - | - |
| kourier-gateway | - | - | - | - |
| net-kourier-controller | - | - | - | - |

### cert-manager Operator: Core Failure Modes

| Component | PK | NP | QE | LS | RR | CD |
|-----------|----|----|----|----|----|----|
| controller | Y | Y | Y | Y | Y | - |
| webhook | Y | Y | Y | Y | - | Y |
| cainjector | Y | Y | Y | Y | - | - |

### cert-manager Operator: Extended Failure Modes

| Component | SD | CL | IC | RD | PB |
|----|----|----|----|----|----|
| cert-manager | Y | Y | Y | Y | Y |

### Strimzi (AMQ Streams): Core Failure Modes

| Component | PK | NP | LS |
|----|----|----|----|
| cluster-operator | Y | Y | Y |

### Strimzi (AMQ Streams): Extended Failure Modes

| Component | SZ | LE | CL | IC | PB |
|----|----|----|----|----|----|
| cluster-operator | Y | Y | Y | Y | Y |

### Spark Operator: Core Failure Modes

| Component | PK | NP | LS |
|----|----|----|----|
| controller | Y | Y | Y |
| webhook | Y | - | - |

### Spark Operator: Extended Failure Modes

| Component | SZ | LE | CL | IC | RD | PB | WD |
|----|----|----|----|----|----|----|----|
| controller | Y | Y | Y | Y | - | Y | - |
| webhook | Y | - | - | - | Y | - | Y |

### ArgoCD (OpenShift GitOps): Core Failure Modes

| Component | PK | NP | LS |
|----|----|----|----|
| server | Y | Y | Y |
| repo-server | Y | Y | Y |

### ArgoCD (OpenShift GitOps): Extended Failure Modes

| Component | SZ | CL | IC | RD | PB |
|----|----|----|----|----|----|
| server | Y | Y | Y | Y | Y |
| repo-server | Y | Y | Y | Y | Y |

### Tekton (OpenShift Pipelines): Core Failure Modes

| Component | PK | NP |
|-----------|----|----|
| pipelines-controller | Y | Y |
| pipelines-webhook | Y | Y |

### Tekton (OpenShift Pipelines): Extended Failure Modes

| Component | SZ | LE | CL | IC | RD | PB |
|-----------|----|----|----|----|----|----|
| pipelines-controller | Y | Y | Y | Y | - | Y |
| pipelines-webhook | Y | - | Y | Y | Y | Y |

### Prometheus Operator: Core Failure Modes

| Component | PK | NP |
|-----------|----|----|
| prometheus-operator | Y | Y |

### Prometheus Operator: Extended Failure Modes

| Component | SZ | CL | IC | PB |
|-----------|----|----|----|----|
| prometheus-operator | Y | Y | Y | Y |
