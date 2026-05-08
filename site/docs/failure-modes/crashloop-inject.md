# CrashLoopInject

**Danger Level:** :material-shield-remove: High

Patches a Deployment's container command to a nonexistent binary, causing CrashLoopBackOff.

## Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | Yes | - | Name of the Deployment to inject |
| `containerName` | `string` | No | first container | Name of the container to target (defaults to the first container in the pod spec) |
| `ttl` | `duration` | No | `300s` | Auto-cleanup duration |

## How It Works

CrashLoopInject replaces the container's `command` field with `/chaos-nonexistent-binary`. This triggers a new rollout where the new pods immediately crash because the binary does not exist, entering `CrashLoopBackOff`. The original command is stored as JSON in a Deployment annotation keyed by container name (`chaos.operatorchaos.io/original-command-<container>`).

**API calls:**
1. `Get` the target Deployment
2. Find the target container (by name, or default to first)
3. Serialize the original command to JSON (or `"null"` for containers using image ENTRYPOINT)
4. `Patch` the Deployment: store original command in annotation, replace command with `["/chaos-nonexistent-binary"]`
5. On revert: `Get` the Deployment, read the stored command annotation, `Patch` to restore (set to `nil` for ENTRYPOINT-based containers), remove annotations

**Cleanup:** Restores the original command (or clears it to nil for containers that relied on the image ENTRYPOINT). Kubernetes then rolls out new pods with the correct command.

**Crash safety:** The original command is stored in the Deployment's annotation, surviving chaos tool crashes. Manual recovery: read the `chaos.operatorchaos.io/original-command-<container>` annotation and patch the command back.

**Comparison with other failure modes:**

- **PodKill** deletes running pods, which are immediately replaced by the Deployment controller with identical pods. CrashLoopInject modifies the pod template, so replacement pods also crash.
- **ImageCorrupt** causes `ImagePullBackOff` (container image cannot be pulled). CrashLoopInject causes `CrashLoopBackOff` (container starts but immediately exits).
- Both CrashLoopInject and ImageCorrupt test rollout resilience, but they exercise different failure detection paths in the operator.

## Disruption Rubric

**Expected behavior on a healthy operator:**
The Deployment's new ReplicaSet creates pods that immediately crash. The old ReplicaSet's pods are terminated based on the rollout strategy (RollingUpdate keeps some old pods alive during the transition). The operator should detect the degraded Deployment (via `Available` or `Progressing` conditions) and report it in status. On revert, the command is restored and a clean rollout proceeds.

**Contract violation indicators:**
- Operator does not detect or report the CrashLoopBackOff state (indicates missing Deployment health monitoring)
- Operator attempts to "fix" the crash by restarting or deleting pods without addressing the root cause (ineffective remediation)
- After revert, old pods are not cleaned up and new pods fail to start (indicates rollout stuck in a bad state)
- Operator's status still shows healthy while all pods are crashing

**Collateral damage risks:**
- **High.** With RollingUpdate strategy (the default), Kubernetes keeps `maxUnavailable` old pods running during the bad rollout, so some capacity may remain. With Recreate strategy, all pods are terminated before the new (crashing) ones start.
- Webhook-serving containers that crash will cause API server errors for validated resources
- If `containerName` targets a sidecar, the main container may still run but in a degraded state

**Recovery expectations:**
- Recovery time: 15-45 seconds after revert (new rollout with correct command)
- Reconcile cycles: 1 (Deployment controller handles the rollout)
- What "recovered" means: all pods are Running and Ready, Deployment has `Available=True`

## Cross-Component Results

| Component | Experiment | Danger | Description |
|-----------|------------|--------|-------------|
| odh-model-controller | odh-model-controller-crashloop-inject | high | Replacing the container command causes CrashLoopBackOff. The Deployment's RollingUpdate strategy keeps the old pod alive. On revert, a clean rollout restores the controller. |
| kserve | kserve-crashloop-inject | high | Replacing the kserve-controller-manager command causes CrashLoopBackOff. The Deployment's RollingUpdate strategy keeps the old pod alive. On revert, a clean rollout restores the controller. |
| knative-serving | knative-serving-controller-crashloop-inject | high | Replacing the knative-serving controller command causes CrashLoopBackOff. The Deployment's RollingUpdate strategy keeps the old pod alive. On revert, a clean rollout restores the controller. |
| cert-manager | cert-manager-crashloop-inject | high | Replacing the cert-manager command causes CrashLoopBackOff. The Deployment's RollingUpdate strategy keeps the old pod alive. On revert, a clean rollout restores the controller. |
| service-mesh | istiod-crashloop-inject | high | Replacing the istiod command causes CrashLoopBackOff. The Deployment's RollingUpdate strategy keeps the old pod alive. On revert, a clean rollout restores the controller. |

<!-- custom-start: notes -->
<!-- custom-end: notes -->
