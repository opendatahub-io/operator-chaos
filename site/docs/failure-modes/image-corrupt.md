# ImageCorrupt

**Danger Level:** :material-shield-remove: High

Patches a Deployment's container image to an invalid registry, causing ImagePullBackOff.

## Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | Yes | - | Name of the Deployment to inject |
| `containerName` | `string` | No | first container | Name of the container to target (defaults to the first container in the pod spec) |
| `image` | `string` | No | `registry.invalid/nonexistent:chaos` | The corrupt image reference to inject |
| `ttl` | `duration` | No | `300s` | Auto-cleanup duration |

## How It Works

ImageCorrupt replaces the container's `image` field with an invalid registry reference (default: `registry.invalid/nonexistent:chaos`). This triggers a new rollout where the new pods cannot pull the image, entering `ImagePullBackOff`. The original image is stored in a Deployment annotation keyed by container name (`chaos.operatorchaos.io/original-image-<container>`).

**API calls:**
1. `Get` the target Deployment
2. Find the target container (by name, or default to first)
3. Store the original image reference in an annotation
4. `Patch` the Deployment: set annotation, replace image with the corrupt value
5. On revert: `Get` the Deployment, read the stored image annotation, `Patch` to restore, remove annotations

**Cleanup:** Restores the original container image. Kubernetes then rolls out new pods that can pull the correct image.

**Crash safety:** The original image is stored in the Deployment's annotation, surviving chaos tool crashes. Manual recovery: read the `chaos.operatorchaos.io/original-image-<container>` annotation and patch the image back.

## Disruption Rubric

**Expected behavior on a healthy operator:**
The Deployment's new ReplicaSet creates pods that fail to pull the image. The kubelet retries with exponential backoff (ImagePullBackOff). The Deployment's `Progressing` condition should eventually become `False` with a `ProgressDeadlineExceeded` reason. The operator should detect this degraded state and report it in status. On revert, the image is restored and a clean rollout proceeds.

**Contract violation indicators:**
- Operator does not detect or report the ImagePullBackOff state (indicates missing Deployment health monitoring)
- Operator does not set a `progressDeadlineSeconds` on its Deployments, preventing Kubernetes from reporting the stuck rollout
- After revert, the rollout does not complete cleanly (indicates corrupted Deployment state)
- Operator's status still shows healthy while pods cannot pull images

**Collateral damage risks:**
- **High.** With RollingUpdate strategy (the default), Kubernetes keeps old pods running during the stuck rollout, preserving some capacity. With Recreate strategy, all pods are terminated before the new (stuck) ones attempt to start.
- The stuck rollout consumes image pull attempts and kubelet resources
- If the Deployment serves webhooks, API server operations depend on whether old pods remain

**Recovery expectations:**
- Recovery time: 15-45 seconds after revert (new rollout with correct image, assuming image is cached on node)
- Reconcile cycles: 1 (Deployment controller handles the rollout)
- What "recovered" means: all pods are Running and Ready, Deployment has `Available=True`

## Cross-Component Results

| Component | Experiment | Danger | Description |
|-----------|------------|--------|-------------|
| odh-model-controller | odh-model-controller-image-corrupt | high | Patching the container image to an invalid registry causes ImagePullBackOff. The Deployment's RollingUpdate strategy keeps the old pod alive. On revert, a clean rollout restores the controller. |
| kserve | kserve-image-corrupt | high | Patching the kserve-controller-manager image to an invalid registry causes ImagePullBackOff. The Deployment's RollingUpdate strategy keeps the old pod alive. On revert, a clean rollout restores the controller. |
| knative-serving | knative-serving-controller-image-corrupt | high | Patching the knative-serving controller image to an invalid registry causes ImagePullBackOff. The Deployment's RollingUpdate strategy keeps the old pod alive. On revert, a clean rollout restores the controller. |
| cert-manager | cert-manager-image-corrupt | high | Patching the cert-manager image to an invalid registry causes ImagePullBackOff. The Deployment's RollingUpdate strategy keeps the old pod alive. On revert, a clean rollout restores the controller. |
| service-mesh | istiod-image-corrupt | high | Patching the istiod image to an invalid registry causes ImagePullBackOff. The Deployment's RollingUpdate strategy keeps the old pod alive. On revert, a clean rollout restores the controller. |

<!-- custom-start: notes -->
<!-- custom-end: notes -->
