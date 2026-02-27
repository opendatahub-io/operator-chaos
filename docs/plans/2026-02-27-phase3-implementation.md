# Phase 3: Full SDK + Plugin Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Address critical issues from Phase 2 architect reviews and implement Phase 3 milestones (M15: advanced fault categories, M16: suite runner + CI integration, M17: controller-mode CRD foundations), deferring M14 (AI plugin) to Phase 4.

**Architecture:** Phase 3 is split into three parts: Part A front-loads critical safety fixes from Phase 2 reviews (rollback state persistence, lease expiry, race condition tests, structured logging, nil-client guards, clean command coverage). Part B implements advanced fault categories (memory, CPU, I/O, concurrency) as SDK fault specs. Part C delivers the suite runner with actual experiment execution, CI integration via exit codes, and CRD-readiness type migrations.

**Tech Stack:** Go 1.25, controller-runtime v0.23.1, k8s.io/api v0.35.1, slog (stdlib), cobra, testify, fake client for tests

---

## Part A: Critical Phase 2 Review Fixes (Tasks 1-14)

These tasks address the highest-priority findings from the 5-architect Phase 2 review council.

---

### Task 1: Rollback State Persistence for RBACRevoke Injector

The RBACRevoke injector stores original subjects only in an in-memory closure. If the process crashes, the original RBAC subjects are permanently lost. This task persists rollback data as annotations on the modified resource.

**Files:**
- Modify: `pkg/injection/rbac.go`
- Modify: `pkg/injection/rbac_test.go`
- Modify: `pkg/safety/rollback.go`

**Step 1: Write the failing test**

Add to `pkg/injection/rbac_test.go`:

```go
func TestRBACRevokeInjectStoresRollbackAnnotation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rbacv1.AddToScheme(scheme))

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-binding",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "sa1", Namespace: "ns1"},
			{Kind: "ServiceAccount", Name: "sa2", Namespace: "ns2"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "test-role",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(crb).Build()
	injector := NewRBACRevokeInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.RBACRevoke,
		Parameters: map[string]string{
			"bindingName": "test-binding",
			"bindingType": "ClusterRoleBinding",
		},
	}

	cleanup, _, err := injector.Inject(context.Background(), spec, "default")
	require.NoError(t, err)

	// Verify the annotation was written with original subjects
	var modified rbacv1.ClusterRoleBinding
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-binding"}, &modified)
	require.NoError(t, err)

	rollbackData, ok := modified.Annotations[safety.RollbackAnnotationKey]
	assert.True(t, ok, "rollback annotation should be set")
	assert.Contains(t, rollbackData, "sa1")
	assert.Contains(t, rollbackData, "sa2")

	// Verify cleanup removes the annotation
	require.NoError(t, cleanup(context.Background()))

	var restored rbacv1.ClusterRoleBinding
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-binding"}, &restored)
	require.NoError(t, err)
	_, hasAnnotation := restored.Annotations[safety.RollbackAnnotationKey]
	assert.False(t, hasAnnotation, "rollback annotation should be removed after cleanup")
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/injection/ -run TestRBACRevokeInjectStoresRollbackAnnotation -v`
Expected: FAIL (no annotation found)

**Step 3: Implement rollback annotation in RBACRevoke**

In `pkg/injection/rbac.go`, modify `injectClusterRoleBinding` to serialize original subjects and store as annotation before clearing:

```go
func (r *RBACRevokeInjector) injectClusterRoleBinding(ctx context.Context, name string) (CleanupFunc, []v1alpha1.InjectionEvent, error) {
	var crb rbacv1.ClusterRoleBinding
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, &crb); err != nil {
		return nil, nil, fmt.Errorf("getting ClusterRoleBinding %s: %w", name, err)
	}

	originalSubjects := crb.Subjects

	// Persist rollback data as annotation
	rollbackData, err := json.Marshal(originalSubjects)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing rollback data: %w", err)
	}
	if crb.Annotations == nil {
		crb.Annotations = make(map[string]string)
	}
	crb.Annotations[safety.RollbackAnnotationKey] = string(rollbackData)

	// Add chaos labels
	if crb.Labels == nil {
		crb.Labels = make(map[string]string)
	}
	for k, v := range safety.ChaosLabels(string(v1alpha1.RBACRevoke)) {
		crb.Labels[k] = v
	}

	// Clear subjects
	crb.Subjects = nil
	if err := r.client.Update(ctx, &crb); err != nil {
		return nil, nil, fmt.Errorf("clearing subjects on ClusterRoleBinding %s: %w", name, err)
	}

	events := []v1alpha1.InjectionEvent{
		NewEvent(v1alpha1.RBACRevoke, name, "subjects-cleared", map[string]string{
			"bindingType":    "ClusterRoleBinding",
			"subjectsStored": "annotation",
		}),
	}

	cleanup := func(ctx context.Context) error {
		var current rbacv1.ClusterRoleBinding
		if err := r.client.Get(ctx, client.ObjectKey{Name: name}, &current); err != nil {
			return fmt.Errorf("getting ClusterRoleBinding for cleanup: %w", err)
		}
		current.Subjects = originalSubjects
		// Remove rollback annotation and chaos labels
		delete(current.Annotations, safety.RollbackAnnotationKey)
		delete(current.Labels, safety.ManagedByLabel)
		delete(current.Labels, safety.ChaosTypeLabel)
		return r.client.Update(ctx, &current)
	}

	return cleanup, events, nil
}
```

Apply the same pattern to `injectRoleBinding`.

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/injection/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/injection/rbac.go pkg/injection/rbac_test.go
git commit -m "fix: persist RBAC rollback state as annotations for crash-safe recovery"
```

---

### Task 2: Rollback State Persistence for WebhookDisrupt Injector

Same crash-safety issue as Task 1 but for ValidatingWebhookConfiguration FailurePolicy values.

**Files:**
- Modify: `pkg/injection/webhook.go`
- Modify: `pkg/injection/webhook_test.go`

**Step 1: Write the failing test**

Add to `pkg/injection/webhook_test.go`:

```go
func TestWebhookDisruptInjectStoresRollbackAnnotation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, admissionregv1.AddToScheme(scheme))

	ignore := admissionregv1.Ignore
	webhook := &admissionregv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-webhook",
		},
		Webhooks: []admissionregv1.ValidatingWebhook{
			{
				Name:          "test.webhook.io",
				FailurePolicy: &ignore,
				ClientConfig:  admissionregv1.WebhookClientConfig{},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(webhook).Build()
	injector := NewWebhookDisruptInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.WebhookDisrupt,
		Parameters: map[string]string{
			"webhookName": "test-webhook",
			"action":      "fail",
		},
	}

	cleanup, _, err := injector.Inject(context.Background(), spec, "default")
	require.NoError(t, err)

	var modified admissionregv1.ValidatingWebhookConfiguration
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-webhook"}, &modified)
	require.NoError(t, err)

	rollbackData, ok := modified.Annotations[safety.RollbackAnnotationKey]
	assert.True(t, ok, "rollback annotation should be set")
	assert.Contains(t, rollbackData, "Ignore")

	require.NoError(t, cleanup(context.Background()))

	var restored admissionregv1.ValidatingWebhookConfiguration
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-webhook"}, &restored)
	require.NoError(t, err)
	_, hasAnnotation := restored.Annotations[safety.RollbackAnnotationKey]
	assert.False(t, hasAnnotation, "rollback annotation should be removed after cleanup")
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/injection/ -run TestWebhookDisruptInjectStoresRollbackAnnotation -v`
Expected: FAIL

**Step 3: Implement rollback annotation in WebhookDisrupt**

In `pkg/injection/webhook.go`, before modifying FailurePolicy values, serialize the original policies map and store as an annotation on the ValidatingWebhookConfiguration. The cleanup function removes the annotation after restoring.

Store a JSON map of `{"webhookEntryName": "originalPolicyValue"}` in the `RollbackAnnotationKey` annotation. Add chaos labels to the resource.

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/injection/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/injection/webhook.go pkg/injection/webhook_test.go
git commit -m "fix: persist webhook rollback state as annotations for crash-safe recovery"
```

---

### Task 3: Lease Auto-Expiry via LeaseDurationSeconds

The Lease-based lock has no auto-expiry. If the process crashes, the lock persists permanently. Add `LeaseDurationSeconds` and staleness checking.

**Files:**
- Modify: `pkg/safety/lease.go`
- Modify: `pkg/safety/lease_test.go`

**Step 1: Write the failing test**

Add to `pkg/safety/lease_test.go`:

```go
func TestLeaseExperimentLockExpiry(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	// Create an already-expired lease
	expiredTime := metav1.NewMicroTime(time.Now().Add(-20 * time.Minute))
	leaseDuration := int32(600) // 10 minutes
	holder := "old-experiment"
	expiredLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odh-chaos-lock-test-operator",
			Namespace: "default",
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "odh-chaos"},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &holder,
			LeaseDurationSeconds: &leaseDuration,
			AcquireTime:          &expiredTime,
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(expiredLease).Build()
	lock := NewLeaseExperimentLock(fakeClient, "default")

	// Should succeed because the existing lease is expired
	err := lock.Acquire(context.Background(), "test-operator", "new-experiment")
	assert.NoError(t, err, "should acquire lock when existing lease is expired")
}

func TestLeaseExperimentLockSetsExpiry(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	lock := NewLeaseExperimentLock(fakeClient, "default")

	err := lock.Acquire(context.Background(), "test-operator", "my-experiment")
	require.NoError(t, err)

	var lease coordinationv1.Lease
	err = fakeClient.Get(context.Background(), client.ObjectKey{
		Name:      "odh-chaos-lock-test-operator",
		Namespace: "default",
	}, &lease)
	require.NoError(t, err)

	assert.NotNil(t, lease.Spec.LeaseDurationSeconds, "lease should have duration set")
	assert.NotNil(t, lease.Spec.AcquireTime, "lease should have acquire time set")
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/safety/ -run TestLeaseExperimentLock -v`
Expected: FAIL

**Step 3: Implement lease expiry**

In `pkg/safety/lease.go`, modify `Acquire`:
1. Set `LeaseDurationSeconds` (default 900 = 15 minutes) and `AcquireTime` when creating a Lease.
2. On `AlreadyExists` error, fetch the existing Lease and check if it's expired (current time > acquireTime + leaseDuration). If expired, delete the stale lease and retry creation.

```go
const DefaultLeaseDurationSeconds = int32(900) // 15 minutes

func (l *LeaseExperimentLock) Acquire(ctx context.Context, operator string, experimentName string) error {
	leaseName := fmt.Sprintf("odh-chaos-lock-%s", operator)
	now := metav1.NewMicroTime(time.Now())
	duration := DefaultLeaseDurationSeconds

	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: l.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "odh-chaos",
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &experimentName,
			LeaseDurationSeconds: &duration,
			AcquireTime:          &now,
		},
	}

	err := l.client.Create(ctx, lease)
	if err == nil {
		return nil
	}

	if !errors.IsAlreadyExists(err) {
		return fmt.Errorf("creating lease %s: %w", leaseName, err)
	}

	// Check if existing lease is expired
	var existing coordinationv1.Lease
	if getErr := l.client.Get(ctx, client.ObjectKey{Name: leaseName, Namespace: l.namespace}, &existing); getErr != nil {
		return fmt.Errorf("checking existing lease: %w", getErr)
	}

	if l.isExpired(&existing) {
		// Delete stale lease and retry
		if delErr := l.client.Delete(ctx, &existing); delErr != nil {
			return fmt.Errorf("deleting expired lease: %w", delErr)
		}
		if createErr := l.client.Create(ctx, lease); createErr != nil {
			return fmt.Errorf("re-creating lease after expiry: %w", createErr)
		}
		return nil
	}

	holder := ""
	if existing.Spec.HolderIdentity != nil {
		holder = *existing.Spec.HolderIdentity
	}
	return fmt.Errorf("operator %s is locked by experiment %q", operator, holder)
}

func (l *LeaseExperimentLock) isExpired(lease *coordinationv1.Lease) bool {
	if lease.Spec.AcquireTime == nil || lease.Spec.LeaseDurationSeconds == nil {
		return false // can't determine expiry, treat as active
	}
	expiry := lease.Spec.AcquireTime.Time.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second)
	return time.Now().After(expiry)
}
```

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/safety/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/safety/lease.go pkg/safety/lease_test.go
git commit -m "fix: add lease auto-expiry to prevent permanent lock after crash"
```

---

### Task 4: Nil-Client Guard for Phase 1 Injectors

Phase 1 injectors (PodKill, CRDMutation, ConfigDrift, NetworkPartition) are registered unconditionally in `run.go` even when `k8sClient` is nil, which can cause nil pointer panics.

**Files:**
- Modify: `internal/cli/run.go`

**Step 1: Write the failing test**

This is a code-reading fix. Verify by inspection that Phase 1 injectors are registered outside the nil guard. No separate test file needed — the fix is to the CLI wiring.

**Step 2: Implement the fix**

In `internal/cli/run.go`, move all injector registrations inside the `if k8sClient != nil` block:

```go
// Register all injectors only when we have a working K8s client
if k8sClient != nil {
	// Phase 1 injectors
	registry.Register(v1alpha1.PodKill, injection.NewPodKillInjector(k8sClient))
	registry.Register(v1alpha1.CRDMutation, injection.NewCRDMutationInjector(k8sClient))
	registry.Register(v1alpha1.ConfigDrift, injection.NewConfigDriftInjector(k8sClient))
	registry.Register(v1alpha1.NetworkPartition, injection.NewNetworkPartitionInjector(k8sClient))

	// Phase 2 injectors
	registry.Register(v1alpha1.WebhookDisrupt, injection.NewWebhookDisruptInjector(k8sClient))
	registry.Register(v1alpha1.RBACRevoke, injection.NewRBACRevokeInjector(k8sClient))
	registry.Register(v1alpha1.FinalizerBlock, injection.NewFinalizerBlockInjector(k8sClient))
}
```

**Step 3: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... 2>&1 | tail -20`
Expected: All PASS

**Step 4: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add internal/cli/run.go
git commit -m "fix: guard all injector registrations behind nil-client check"
```

---

### Task 5: Expand Clean Command for WebhookDisrupt and RBACRevoke

The `clean` command only removes chaos-created resources (NetworkPolicies, Leases). It does not restore chaos-modified resources (webhooks, RBAC bindings). Now that Tasks 1-2 added rollback annotations, `clean` can read and restore them.

**Files:**
- Modify: `internal/cli/clean.go`

**Step 1: Implement webhook cleanup**

Add to `internal/cli/clean.go`:

```go
func cleanWebhookConfigurations(ctx context.Context, k8sClient client.Client) {
	var webhookList admissionregv1.ValidatingWebhookConfigurationList
	if err := k8sClient.List(ctx, &webhookList); err != nil {
		fmt.Printf("  Warning: could not list ValidatingWebhookConfigurations: %v\n", err)
		return
	}

	restored := 0
	for i := range webhookList.Items {
		wh := &webhookList.Items[i]
		rollbackData, ok := wh.Annotations[safety.RollbackAnnotationKey]
		if !ok {
			continue
		}

		// Parse and restore original failure policies
		var originalPolicies map[string]string
		if err := json.Unmarshal([]byte(rollbackData), &originalPolicies); err != nil {
			fmt.Printf("  Warning: could not parse rollback data for webhook %s: %v\n", wh.Name, err)
			continue
		}

		for j := range wh.Webhooks {
			if policyStr, ok := originalPolicies[wh.Webhooks[j].Name]; ok {
				policy := admissionregv1.FailurePolicyType(policyStr)
				wh.Webhooks[j].FailurePolicy = &policy
			}
		}

		// Remove rollback annotation and chaos labels
		delete(wh.Annotations, safety.RollbackAnnotationKey)
		delete(wh.Labels, safety.ManagedByLabel)
		delete(wh.Labels, safety.ChaosTypeLabel)

		if err := k8sClient.Update(ctx, wh); err != nil {
			fmt.Printf("  Warning: could not restore webhook %s: %v\n", wh.Name, err)
			continue
		}
		restored++
	}

	if restored > 0 {
		fmt.Printf("  Restored %d ValidatingWebhookConfiguration(s)\n", restored)
	}
}
```

**Step 2: Implement RBAC cleanup**

Add similar function `cleanRBACBindings` that scans ClusterRoleBindings and RoleBindings for the rollback annotation, deserializes original subjects, and restores them.

**Step 3: Wire into runClean**

Call `cleanWebhookConfigurations` and `cleanRBACBindings` from the main `runClean` function.

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... 2>&1 | tail -20`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add internal/cli/clean.go
git commit -m "feat: expand clean command to restore webhook and RBAC modifications"
```

---

### Task 6: Use ChaosLabels() Consistently in NetworkPartitionInjector

NetworkPartitionInjector hardcodes labels instead of using `safety.ChaosLabels()`. If label values change, this injector would diverge.

**Files:**
- Modify: `pkg/injection/network.go`
- Modify: `pkg/injection/network_test.go`

**Step 1: Write a test that checks label consistency**

Add to `pkg/injection/network_test.go`:

```go
func TestNetworkPartitionInjectUsesChaosLabels(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, networkingv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewNetworkPartitionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.NetworkPartition,
		Parameters: map[string]string{
			"targetLabel": "app=test",
		},
	}

	cleanup, _, err := injector.Inject(context.Background(), spec, "default")
	require.NoError(t, err)
	defer cleanup(context.Background())

	var policies networkingv1.NetworkPolicyList
	err = fakeClient.List(context.Background(), &policies, client.InNamespace("default"))
	require.NoError(t, err)
	require.Len(t, policies.Items, 1)

	expectedLabels := safety.ChaosLabels(string(v1alpha1.NetworkPartition))
	for k, v := range expectedLabels {
		assert.Equal(t, v, policies.Items[0].Labels[k], "label %s should match ChaosLabels()", k)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/injection/ -run TestNetworkPartitionInjectUsesChaosLabels -v`
Expected: FAIL (labels don't include chaos type label)

**Step 3: Update network.go to use ChaosLabels()**

Replace hardcoded label map with `safety.ChaosLabels(string(v1alpha1.NetworkPartition))`.

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/injection/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/injection/network.go pkg/injection/network_test.go
git commit -m "fix: use ChaosLabels() consistently in NetworkPartitionInjector"
```

---

### Task 7: Race Condition Tests for FaultConfig

Zero concurrent tests exist despite `sync.RWMutex` in `FaultConfig`. Add tests verifying thread safety.

**Files:**
- Modify: `pkg/sdk/types_test.go`

**Step 1: Write the race condition test**

Add to `pkg/sdk/types_test.go`:

```go
func TestFaultConfigConcurrentAccess(t *testing.T) {
	fc := &FaultConfig{
		Active: true,
		Faults: map[string]FaultSpec{
			"get": {ErrorRate: 0.5, Error: "chaos"},
		},
	}

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	// Concurrent readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = fc.MaybeInject("get")
				_ = fc.IsActive()
			}
		}()
	}

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				fc.mu.Lock()
				fc.Active = !fc.Active
				fc.mu.Unlock()
			}
		}()
	}

	wg.Wait()
	close(errs)

	// If we get here without a race detector failure, the test passes
	// Run with: go test -race ./pkg/sdk/ -run TestFaultConfigConcurrentAccess
}

func TestLocalExperimentLockConcurrentAccess(t *testing.T) {
	lock := NewLocalExperimentLock()

	var wg sync.WaitGroup
	acquired := int32(0)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := lock.Acquire(context.Background(), "test-op", fmt.Sprintf("exp-%d", id))
			if err == nil {
				atomic.AddInt32(&acquired, 1)
				time.Sleep(time.Millisecond)
				lock.Release("test-op")
			}
		}(i)
	}

	wg.Wait()
	assert.Greater(t, atomic.LoadInt32(&acquired), int32(0), "at least one goroutine should acquire lock")
}
```

**Step 2: Run with race detector**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test -race ./pkg/sdk/ -run TestFaultConfigConcurrentAccess -v`
Expected: PASS (no race detected)

**Step 3: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/sdk/types_test.go
git commit -m "test: add race condition tests for FaultConfig concurrent access"
```

---

### Task 8: Race Condition Test for LocalExperimentLock

Add the concurrent test for `localExperimentLock` in the safety package.

**Files:**
- Modify: `pkg/safety/mutex_test.go`

**Step 1: Write the concurrent lock test**

Add to `pkg/safety/mutex_test.go`:

```go
func TestLocalExperimentLockConcurrentAccess(t *testing.T) {
	lock := NewLocalExperimentLock()

	var wg sync.WaitGroup
	acquired := int32(0)
	conflicts := int32(0)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := lock.Acquire(context.Background(), "test-op", fmt.Sprintf("exp-%d", id))
			if err == nil {
				atomic.AddInt32(&acquired, 1)
				time.Sleep(time.Millisecond)
				lock.Release("test-op")
			} else {
				atomic.AddInt32(&conflicts, 1)
			}
		}(i)
	}

	wg.Wait()
	total := atomic.LoadInt32(&acquired) + atomic.LoadInt32(&conflicts)
	assert.Equal(t, int32(20), total, "all goroutines should complete")
	assert.Greater(t, atomic.LoadInt32(&acquired), int32(0), "at least one should acquire")
}
```

**Step 2: Run with race detector**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test -race ./pkg/safety/ -run TestLocalExperimentLockConcurrentAccess -v`
Expected: PASS

**Step 3: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/safety/mutex_test.go
git commit -m "test: add race condition tests for LocalExperimentLock"
```

---

### Task 9: Structured Logging with slog

Replace `fmt.Fprintf` logging with Go's stdlib `log/slog` for structured, leveled logging.

**Files:**
- Modify: `pkg/orchestrator/lifecycle.go`
- Modify: `pkg/orchestrator/lifecycle_test.go`

**Step 1: Write the test for structured logging**

Add to `pkg/orchestrator/lifecycle_test.go`:

```go
func TestOrchestratorStructuredLogging(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	registry := injection.NewRegistry()
	mockInj := &mockInjector{}
	registry.Register(v1alpha1.PodKill, mockInj)

	orch := New(OrchestratorConfig{
		Registry:  registry,
		Observer:  &mockObserver{result: &v1alpha1.CheckResult{Passed: true}},
		Evaluator: evaluator.NewEvaluator(),
		Lock:      safety.NewLocalExperimentLock(),
		Verbose:   true,
		Logger:    logger,
	})

	exp := &v1alpha1.ChaosExperiment{
		Metadata: v1alpha1.Metadata{Name: "test-structured-log"},
		Spec: v1alpha1.ChaosExperimentSpec{
			Target:      v1alpha1.TargetSpec{Operator: "test", Component: "comp"},
			Injection:   v1alpha1.InjectionSpec{Type: v1alpha1.PodKill},
			BlastRadius: v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1, AllowedNamespaces: []string{"opendatahub"}},
			Hypothesis:  v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 10 * time.Second}},
		},
	}

	_, err := orch.Run(context.Background(), exp)
	require.NoError(t, err)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "test-structured-log")
	assert.Contains(t, logOutput, "phase")
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/orchestrator/ -run TestOrchestratorStructuredLogging -v`
Expected: FAIL (no Logger field on OrchestratorConfig)

**Step 3: Implement structured logging**

Modify `OrchestratorConfig` and `Orchestrator` to accept an `*slog.Logger`. Replace `o.log()` calls with `o.logger.Info()` / `o.logger.Warn()` calls with structured fields. Keep backward compatibility: if `Logger` is nil, create a default text handler writing to `o.output`.

```go
import "log/slog"

type OrchestratorConfig struct {
	// ... existing fields ...
	Logger *slog.Logger
}

type Orchestrator struct {
	// ... existing fields ...
	logger *slog.Logger
}

func New(config OrchestratorConfig) *Orchestrator {
	output := io.Writer(os.Stdout)
	logger := config.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(output, nil))
	}
	return &Orchestrator{
		// ... existing fields ...
		logger: logger,
	}
}
```

Replace `o.log("Phase: PENDING - validating experiment %s", exp.Metadata.Name)` with:
```go
o.logger.Info("phase transition", "phase", "PENDING", "experiment", exp.Metadata.Name, "action", "validating")
```

Gate all logging behind `o.verbose` check.

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/orchestrator/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/orchestrator/lifecycle.go pkg/orchestrator/lifecycle_test.go
git commit -m "feat: replace fmt.Fprintf logging with structured slog"
```

---

### Task 10: Add Inject() Tests for PodKill Injector

PodKill has 0% coverage on its `Inject()` method — only validation is tested.

**Files:**
- Modify: `pkg/injection/podkill_test.go`

**Step 1: Write the Inject tests**

Add to `pkg/injection/podkill_test.go`:

```go
func TestPodKillInjectSuccessful(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
			UID:       "uid-1",
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
			UID:       "uid-2",
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod1, pod2).Build()
	injector := NewPodKillInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:       v1alpha1.PodKill,
		Count:      1,
		Parameters: map[string]string{"labelSelector": "app=test"},
	}
	blast := v1alpha1.BlastRadiusSpec{MaxPodsAffected: 2}

	cleanup, events, err := injector.Inject(context.Background(), spec, "default")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.NotNil(t, cleanup)

	// Verify one pod was deleted
	var pods corev1.PodList
	err = fakeClient.List(context.Background(), &pods, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Len(t, pods.Items, 1)
}

func TestPodKillInjectNoPodsFound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewPodKillInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:       v1alpha1.PodKill,
		Count:      1,
		Parameters: map[string]string{"labelSelector": "app=nonexistent"},
	}

	_, _, err := injector.Inject(context.Background(), spec, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found")
}

func TestPodKillInjectCountExceedsAvailable(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
			UID:       "uid-1",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod1).Build()
	injector := NewPodKillInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type:       v1alpha1.PodKill,
		Count:      5,
		Parameters: map[string]string{"labelSelector": "app=test"},
	}

	cleanup, events, err := injector.Inject(context.Background(), spec, "default")
	require.NoError(t, err)
	assert.Len(t, events, 1, "should cap to available pods")
	assert.NotNil(t, cleanup)
}
```

**Step 2: Run tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/injection/ -run TestPodKillInject -v`
Expected: All PASS

**Step 3: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/injection/podkill_test.go
git commit -m "test: add Inject() method tests for PodKillInjector"
```

---

### Task 11: Add Inject() Tests for NetworkPartition and ConfigDrift Injectors

Both have 0% coverage on Inject().

**Files:**
- Modify: `pkg/injection/network_test.go`
- Modify: `pkg/injection/configdrift_test.go`

**Step 1: Write NetworkPartition Inject test**

Add to `pkg/injection/network_test.go`:

```go
func TestNetworkPartitionInjectAndCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, networkingv1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	injector := NewNetworkPartitionInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.NetworkPartition,
		Parameters: map[string]string{
			"targetLabel": "app=test",
		},
	}

	cleanup, events, err := injector.Inject(context.Background(), spec, "default")
	require.NoError(t, err)
	assert.NotEmpty(t, events)
	assert.NotNil(t, cleanup)

	// Verify NetworkPolicy was created
	var policies networkingv1.NetworkPolicyList
	err = fakeClient.List(context.Background(), &policies, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Len(t, policies.Items, 1)

	// Verify cleanup removes it
	require.NoError(t, cleanup(context.Background()))

	err = fakeClient.List(context.Background(), &policies, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Len(t, policies.Items, 0)
}
```

**Step 2: Write ConfigDrift Inject test**

Add to `pkg/injection/configdrift_test.go`:

```go
func TestConfigDriftInjectAndCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"key1": "original-value",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	injector := NewConfigDriftInjector(fakeClient)

	spec := v1alpha1.InjectionSpec{
		Type: v1alpha1.ConfigDrift,
		Parameters: map[string]string{
			"resourceType": "ConfigMap",
			"name":         "test-config",
			"key":          "key1",
			"value":        "drifted-value",
		},
	}

	cleanup, events, err := injector.Inject(context.Background(), spec, "default")
	require.NoError(t, err)
	assert.NotEmpty(t, events)

	// Verify value was changed
	var modified corev1.ConfigMap
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-config", Namespace: "default"}, &modified)
	require.NoError(t, err)
	assert.Equal(t, "drifted-value", modified.Data["key1"])

	// Verify cleanup restores original
	require.NoError(t, cleanup(context.Background()))

	var restored corev1.ConfigMap
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-config", Namespace: "default"}, &restored)
	require.NoError(t, err)
	assert.Equal(t, "original-value", restored.Data["key1"])
}
```

**Step 3: Run tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/injection/ -v`
Expected: All PASS

**Step 4: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/injection/network_test.go pkg/injection/configdrift_test.go
git commit -m "test: add Inject() method tests for NetworkPartition and ConfigDrift injectors"
```

---

### Task 12: Add ChaosClient CRUD Coverage Tests

ChaosClient only tests `Get`. Add tests for `List`, `Create`, `Update`, `Delete`, `Patch`, `DeleteAllOf`.

**Files:**
- Modify: `pkg/sdk/client_test.go`

**Step 1: Write tests for remaining operations**

Add to `pkg/sdk/client_test.go`:

```go
func TestChaosClientFaultInjectionOnAllOperations(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	operations := []struct {
		name      string
		faultKey  string
		callFunc  func(c *ChaosClient) error
	}{
		{
			name:     "List",
			faultKey: "list",
			callFunc: func(c *ChaosClient) error {
				return c.List(context.Background(), &corev1.PodList{})
			},
		},
		{
			name:     "Create",
			faultKey: "create",
			callFunc: func(c *ChaosClient) error {
				return c.Create(context.Background(), &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				})
			},
		},
		{
			name:     "Update",
			faultKey: "update",
			callFunc: func(c *ChaosClient) error {
				return c.Update(context.Background(), &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				})
			},
		},
		{
			name:     "Delete",
			faultKey: "delete",
			callFunc: func(c *ChaosClient) error {
				return c.Delete(context.Background(), &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				})
			},
		},
		{
			name:     "Patch",
			faultKey: "patch",
			callFunc: func(c *ChaosClient) error {
				return c.Patch(context.Background(), &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				}, client.MergeFrom(&corev1.ConfigMap{}))
			},
		},
	}

	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			faults := &FaultConfig{
				Active: true,
				Faults: map[string]FaultSpec{
					op.faultKey: {ErrorRate: 1.0, Error: "chaos-" + op.faultKey},
				},
			}
			chaosClient := NewChaosClient(fakeClient, faults)

			err := op.callFunc(chaosClient)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "chaos-"+op.faultKey)
		})
	}
}
```

**Step 2: Run tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/ -run TestChaosClientFaultInjectionOnAllOperations -v`
Expected: All PASS

**Step 3: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/sdk/client_test.go
git commit -m "test: add ChaosClient fault injection tests for all CRUD operations"
```

---

### Task 13: Define Typed Operation Constants

Operation names ("get", "list", "create") are stringly-typed in FaultConfig.Faults map. Define constants to prevent typos.

**Files:**
- Modify: `pkg/sdk/types.go`
- Modify: `pkg/sdk/client.go`
- Modify: `pkg/sdk/types_test.go`

**Step 1: Write test using typed constants**

Add to `pkg/sdk/types_test.go`:

```go
func TestOperationConstants(t *testing.T) {
	ops := []Operation{OpGet, OpList, OpCreate, OpUpdate, OpDelete, OpPatch, OpDeleteAllOf, OpReconcile}
	for _, op := range ops {
		assert.NotEmpty(t, string(op))
	}

	// Verify constants match what client.go uses
	fc := &FaultConfig{
		Active: true,
		Faults: map[string]FaultSpec{
			string(OpGet): {ErrorRate: 1.0, Error: "test"},
		},
	}
	err := fc.MaybeInject(string(OpGet))
	assert.Error(t, err)
}
```

**Step 2: Define the Operation type and constants**

Add to `pkg/sdk/types.go`:

```go
// Operation represents a client operation that can be fault-injected.
type Operation string

const (
	OpGet        Operation = "get"
	OpList       Operation = "list"
	OpCreate     Operation = "create"
	OpUpdate     Operation = "update"
	OpDelete     Operation = "delete"
	OpPatch      Operation = "patch"
	OpDeleteAllOf Operation = "deleteAllOf"
	OpReconcile  Operation = "reconcile"
)
```

Update `client.go` to use `string(OpGet)` etc. instead of raw strings.

**Step 3: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/ -v`
Expected: All PASS

**Step 4: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/sdk/types.go pkg/sdk/client.go pkg/sdk/types_test.go
git commit -m "feat: define typed Operation constants for fault injection keys"
```

---

### Task 14: Cleanup Failure Surfacing in ExperimentResult

Cleanup failures are logged but silently discarded. Surface them in the ExperimentResult so reports capture cleanup problems.

**Files:**
- Modify: `pkg/orchestrator/lifecycle.go`
- Modify: `pkg/orchestrator/lifecycle_test.go`

**Step 1: Write the failing test**

Add to `pkg/orchestrator/lifecycle_test.go`:

```go
func TestOrchestratorCleanupFailureSurfacedInResult(t *testing.T) {
	registry := injection.NewRegistry()
	cleanupErr := fmt.Errorf("API server unreachable")
	mockInj := &mockInjector{
		cleanupFunc: func(ctx context.Context) error {
			return cleanupErr
		},
	}
	registry.Register(v1alpha1.PodKill, mockInj)

	orch := New(OrchestratorConfig{
		Registry:  registry,
		Observer:  &mockObserver{result: &v1alpha1.CheckResult{Passed: true}},
		Evaluator: evaluator.NewEvaluator(),
		Lock:      safety.NewLocalExperimentLock(),
	})

	exp := &v1alpha1.ChaosExperiment{
		Metadata: v1alpha1.Metadata{Name: "test-cleanup-fail"},
		Spec: v1alpha1.ChaosExperimentSpec{
			Target:      v1alpha1.TargetSpec{Operator: "test", Component: "comp"},
			Injection:   v1alpha1.InjectionSpec{Type: v1alpha1.PodKill},
			BlastRadius: v1alpha1.BlastRadiusSpec{MaxPodsAffected: 1, AllowedNamespaces: []string{"opendatahub"}},
			Hypothesis:  v1alpha1.HypothesisSpec{RecoveryTimeout: v1alpha1.Duration{Duration: 10 * time.Second}},
		},
	}

	result, err := orch.Run(context.Background(), exp)
	require.NoError(t, err)
	assert.Contains(t, result.CleanupError, "API server unreachable")
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL (no `CleanupError` field on ExperimentResult)

**Step 3: Implement cleanup error surfacing**

Add `CleanupError string` field to `ExperimentResult`. In the deferred cleanup, if `cleanErr != nil`, set `result.CleanupError = cleanErr.Error()`.

```go
type ExperimentResult struct {
	Experiment   string                      `json:"experiment"`
	Phase        v1alpha1.ExperimentPhase    `json:"phase"`
	Verdict      v1alpha1.Verdict            `json:"verdict,omitempty"`
	Evaluation   *evaluator.EvaluationResult `json:"evaluation,omitempty"`
	Report       *reporter.ExperimentReport  `json:"report,omitempty"`
	Error        string                      `json:"error,omitempty"`
	CleanupError string                      `json:"cleanupError,omitempty"`
}
```

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/orchestrator/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/orchestrator/lifecycle.go pkg/orchestrator/lifecycle_test.go
git commit -m "feat: surface cleanup failures in ExperimentResult"
```

---

## Part B: Advanced Fault Categories (Tasks 15-20)

These tasks implement M15 (Memory, CPU, I/O, Concurrency fault categories) as SDK fault specs.

---

### Task 15: Define Phase 3 Injection Type Constants

Add the Phase 3 injection type constants from the design document.

**Files:**
- Modify: `api/v1alpha1/types.go`
- Modify: `api/v1alpha1/types_test.go`

**Step 1: Write the test**

Add to `api/v1alpha1/types_test.go`:

```go
func TestPhase3InjectionTypes(t *testing.T) {
	phase3Types := []InjectionType{
		MemoryLeak, MemoryPressure, GoroutineBomb,
		CPUSpin, FDExhaustion, DiskWriteFailure,
		DNSFailure, DeadlockInject,
	}
	for _, it := range phase3Types {
		assert.NotEmpty(t, string(it))
	}
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL (undefined constants)

**Step 3: Add the constants**

Add to `api/v1alpha1/types.go`:

```go
// Phase 3 injection types (advanced fault categories)
const (
	MemoryLeak       InjectionType = "MemoryLeak"
	MemoryPressure   InjectionType = "MemoryPressure"
	GoroutineBomb    InjectionType = "GoroutineBomb"
	CPUSpin          InjectionType = "CPUSpin"
	FDExhaustion     InjectionType = "FDExhaustion"
	DiskWriteFailure InjectionType = "DiskWriteFailure"
	DNSFailure       InjectionType = "DNSFailure"
	DeadlockInject   InjectionType = "DeadlockInject"
)
```

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./api/... -v`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add api/v1alpha1/types.go api/v1alpha1/types_test.go
git commit -m "feat: add Phase 3 injection type constants for advanced fault categories"
```

---

### Task 16: Memory Fault Specs (MemoryLeak, MemoryPressure, AllocSpike)

SDK-level fault specs for memory-related faults.

**Files:**
- Create: `pkg/sdk/faults/memory.go`
- Create: `pkg/sdk/faults/memory_test.go`

**Step 1: Write the tests**

Create `pkg/sdk/faults/memory_test.go`:

```go
package faults

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryLeakConfig(t *testing.T) {
	spec := MemoryLeakConfig(1024*1024, 100*time.Millisecond)
	assert.Equal(t, 1.0, spec.ErrorRate)
	assert.Contains(t, spec.Error, "memory leak")
	assert.Equal(t, 100*time.Millisecond, spec.Delay)
}

func TestMemoryPressureConfig(t *testing.T) {
	spec := MemoryPressureConfig(0.8)
	assert.Equal(t, 0.8, spec.ErrorRate)
	assert.Contains(t, spec.Error, "memory pressure")
}

func TestAllocSpikeConfig(t *testing.T) {
	spec := AllocSpikeConfig(0.5, 10*1024*1024)
	assert.Equal(t, 0.5, spec.ErrorRate)
	assert.Contains(t, spec.Error, "allocation spike")
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/faults/ -run TestMemory -v`
Expected: FAIL (undefined functions)

**Step 3: Implement memory fault specs**

Create `pkg/sdk/faults/memory.go`:

```go
package faults

import (
	"fmt"
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk"
)

// MemoryLeakConfig creates a fault that simulates memory leak pressure.
// The delay parameter simulates allocation stalls; bytes indicates leak size.
func MemoryLeakConfig(bytes int64, delay time.Duration) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: 1.0,
		Error:     fmt.Sprintf("memory leak injected: %d bytes", bytes),
		Delay:     delay,
	}
}

// MemoryPressureConfig creates a fault that simulates memory pressure errors.
// The errorRate controls how often operations fail due to memory pressure (0.0-1.0).
func MemoryPressureConfig(errorRate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: errorRate,
		Error:     "memory pressure: allocation failed",
	}
}

// AllocSpikeConfig creates a fault that simulates sudden allocation spikes.
func AllocSpikeConfig(errorRate float64, spikeBytes int64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: errorRate,
		Error:     fmt.Sprintf("allocation spike: %d bytes requested", spikeBytes),
	}
}
```

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/faults/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/sdk/faults/memory.go pkg/sdk/faults/memory_test.go
git commit -m "feat: add memory fault category specs (MemoryLeak, MemoryPressure, AllocSpike)"
```

---

### Task 17: CPU Fault Specs (GoroutineBomb, BusySpin, GCPressure)

**Files:**
- Create: `pkg/sdk/faults/cpu.go`
- Create: `pkg/sdk/faults/cpu_test.go`

**Step 1: Write the tests**

Create `pkg/sdk/faults/cpu_test.go`:

```go
package faults

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGoroutineBombConfig(t *testing.T) {
	spec := GoroutineBombConfig(1000)
	assert.Equal(t, 1.0, spec.ErrorRate)
	assert.Contains(t, spec.Error, "goroutine bomb")
}

func TestBusySpinConfig(t *testing.T) {
	spec := BusySpinConfig(500 * time.Millisecond)
	assert.Equal(t, 1.0, spec.ErrorRate)
	assert.Equal(t, 500*time.Millisecond, spec.Delay)
	assert.Contains(t, spec.Error, "CPU spin")
}

func TestGCPressureConfig(t *testing.T) {
	spec := GCPressureConfig(0.7)
	assert.Equal(t, 0.7, spec.ErrorRate)
	assert.Contains(t, spec.Error, "GC pressure")
}
```

**Step 2: Implement CPU fault specs**

Create `pkg/sdk/faults/cpu.go`:

```go
package faults

import (
	"fmt"
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk"
)

// GoroutineBombConfig creates a fault that simulates goroutine explosion.
func GoroutineBombConfig(count int) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: 1.0,
		Error:     fmt.Sprintf("goroutine bomb: %d goroutines spawned", count),
	}
}

// BusySpinConfig creates a fault that simulates CPU-bound busy waiting.
func BusySpinConfig(duration time.Duration) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: 1.0,
		Error:     fmt.Sprintf("CPU spin: busy for %s", duration),
		Delay:     duration,
	}
}

// GCPressureConfig creates a fault that simulates GC pressure.
func GCPressureConfig(errorRate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: errorRate,
		Error:     "GC pressure: excessive garbage collection",
	}
}
```

**Step 3: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/faults/ -v`
Expected: All PASS

**Step 4: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/sdk/faults/cpu.go pkg/sdk/faults/cpu_test.go
git commit -m "feat: add CPU fault category specs (GoroutineBomb, BusySpin, GCPressure)"
```

---

### Task 18: I/O Fault Specs (FDExhaustion, DiskWriteFailure, SlowReader)

**Files:**
- Create: `pkg/sdk/faults/io.go`
- Create: `pkg/sdk/faults/io_test.go`

**Step 1: Write the tests**

Create `pkg/sdk/faults/io_test.go`:

```go
package faults

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFDExhaustionConfig(t *testing.T) {
	spec := FDExhaustionConfig(1024)
	assert.Equal(t, 1.0, spec.ErrorRate)
	assert.Contains(t, spec.Error, "file descriptor")
}

func TestDiskWriteFailureConfig(t *testing.T) {
	spec := DiskWriteFailureConfig(0.9)
	assert.Equal(t, 0.9, spec.ErrorRate)
	assert.Contains(t, spec.Error, "disk write")
}

func TestSlowReaderConfig(t *testing.T) {
	spec := SlowReaderConfig(2 * time.Second)
	assert.Equal(t, 1.0, spec.ErrorRate)
	assert.Equal(t, 2*time.Second, spec.Delay)
	assert.Contains(t, spec.Error, "slow reader")
}
```

**Step 2: Implement I/O fault specs**

Create `pkg/sdk/faults/io.go`:

```go
package faults

import (
	"fmt"
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk"
)

// FDExhaustionConfig creates a fault that simulates file descriptor exhaustion.
func FDExhaustionConfig(maxFDs int) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: 1.0,
		Error:     fmt.Sprintf("file descriptor exhaustion: %d FDs consumed", maxFDs),
	}
}

// DiskWriteFailureConfig creates a fault that simulates disk write failures.
func DiskWriteFailureConfig(errorRate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: errorRate,
		Error:     "disk write failure: I/O error",
	}
}

// SlowReaderConfig creates a fault that simulates slow I/O reads.
func SlowReaderConfig(readDelay time.Duration) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: 1.0,
		Error:     fmt.Sprintf("slow reader: %s delay per read", readDelay),
		Delay:     readDelay,
	}
}
```

**Step 3: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/faults/ -v`
Expected: All PASS

**Step 4: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/sdk/faults/io.go pkg/sdk/faults/io_test.go
git commit -m "feat: add I/O fault category specs (FDExhaustion, DiskWriteFailure, SlowReader)"
```

---

### Task 19: Concurrency Fault Specs (DeadlockInject, ChannelBlock, MutexStarvation)

**Files:**
- Create: `pkg/sdk/faults/concurrency.go`
- Create: `pkg/sdk/faults/concurrency_test.go`

**Step 1: Write the tests**

Create `pkg/sdk/faults/concurrency_test.go`:

```go
package faults

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeadlockInjectConfig(t *testing.T) {
	spec := DeadlockInjectConfig()
	assert.Equal(t, 1.0, spec.ErrorRate)
	assert.Contains(t, spec.Error, "deadlock")
}

func TestChannelBlockConfig(t *testing.T) {
	spec := ChannelBlockConfig(5 * time.Second)
	assert.Equal(t, 1.0, spec.ErrorRate)
	assert.Equal(t, 5*time.Second, spec.Delay)
	assert.Contains(t, spec.Error, "channel block")
}

func TestMutexStarvationConfig(t *testing.T) {
	spec := MutexStarvationConfig(0.6, 1*time.Second)
	assert.Equal(t, 0.6, spec.ErrorRate)
	assert.Equal(t, 1*time.Second, spec.Delay)
	assert.Contains(t, spec.Error, "mutex starvation")
}
```

**Step 2: Implement concurrency fault specs**

Create `pkg/sdk/faults/concurrency.go`:

```go
package faults

import (
	"fmt"
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk"
)

// DeadlockInjectConfig creates a fault that simulates a deadlock condition.
func DeadlockInjectConfig() sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: 1.0,
		Error:     "deadlock injected: resource contention detected",
	}
}

// ChannelBlockConfig creates a fault that simulates a blocked channel.
func ChannelBlockConfig(blockDuration time.Duration) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: 1.0,
		Error:     fmt.Sprintf("channel block: blocked for %s", blockDuration),
		Delay:     blockDuration,
	}
}

// MutexStarvationConfig creates a fault that simulates mutex starvation.
func MutexStarvationConfig(errorRate float64, holdDuration time.Duration) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: errorRate,
		Error:     fmt.Sprintf("mutex starvation: held for %s", holdDuration),
		Delay:     holdDuration,
	}
}
```

**Step 3: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/faults/ -v`
Expected: All PASS

**Step 4: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/sdk/faults/concurrency.go pkg/sdk/faults/concurrency_test.go
git commit -m "feat: add concurrency fault category specs (DeadlockInject, ChannelBlock, MutexStarvation)"
```

---

### Task 20: Network Advanced Fault Specs (ConnectionPoolExhaust, DNSFailure, SocketTimeout)

**Files:**
- Create: `pkg/sdk/faults/network.go`
- Create: `pkg/sdk/faults/network_test.go`

**Step 1: Write the tests**

Create `pkg/sdk/faults/network_test.go`:

```go
package faults

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConnectionPoolExhaustConfig(t *testing.T) {
	spec := ConnectionPoolExhaustConfig(100)
	assert.Equal(t, 1.0, spec.ErrorRate)
	assert.Contains(t, spec.Error, "connection pool")
}

func TestDNSFailureConfig(t *testing.T) {
	spec := DNSFailureConfig(0.8)
	assert.Equal(t, 0.8, spec.ErrorRate)
	assert.Contains(t, spec.Error, "DNS")
}

func TestSocketTimeoutConfig(t *testing.T) {
	spec := SocketTimeoutConfig(3 * time.Second)
	assert.Equal(t, 1.0, spec.ErrorRate)
	assert.Equal(t, 3*time.Second, spec.Delay)
	assert.Contains(t, spec.Error, "socket timeout")
}
```

**Step 2: Implement network fault specs**

Create `pkg/sdk/faults/network.go`:

```go
package faults

import (
	"fmt"
	"time"

	"github.com/opendatahub-io/odh-platform-chaos/pkg/sdk"
)

// ConnectionPoolExhaustConfig creates a fault that simulates connection pool exhaustion.
func ConnectionPoolExhaustConfig(maxConns int) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: 1.0,
		Error:     fmt.Sprintf("connection pool exhausted: %d connections consumed", maxConns),
	}
}

// DNSFailureConfig creates a fault that simulates DNS resolution failures.
func DNSFailureConfig(errorRate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: errorRate,
		Error:     "DNS resolution failed: no such host",
	}
}

// SocketTimeoutConfig creates a fault that simulates socket timeouts.
func SocketTimeoutConfig(timeout time.Duration) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: 1.0,
		Error:     fmt.Sprintf("socket timeout after %s", timeout),
		Delay:     timeout,
	}
}
```

**Step 3: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/faults/ -v`
Expected: All PASS

**Step 4: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/sdk/faults/network.go pkg/sdk/faults/network_test.go
git commit -m "feat: add network advanced fault specs (ConnectionPoolExhaust, DNSFailure, SocketTimeout)"
```

---

## Part C: Suite Runner + CRD Foundations (Tasks 21-26)

---

### Task 21: Make Suite Command Execute Experiments

The `suite` command currently only validates but prints "WOULD RUN" even when `--dry-run=false`. Wire it to actually execute experiments through the orchestrator.

**Files:**
- Modify: `internal/cli/suite.go`
- Modify: `internal/cli/run.go` (extract shared orchestrator setup)

**Step 1: Extract orchestrator setup from run.go**

Create a shared function `buildOrchestrator` that both `run` and `suite` can use. Move the K8s client creation, registry setup, observer creation, and orchestrator assembly into this function.

Add to `internal/cli/run.go`:

```go
// OrchestratorBundle holds all components needed to run an experiment.
type OrchestratorBundle struct {
	Orchestrator *orchestrator.Orchestrator
	Knowledge    *model.OperatorKnowledge
}

func buildOrchestrator(cmd *cobra.Command, knowledgePath string, dryRun bool, reportDir string, timeout time.Duration) (*OrchestratorBundle, error) {
	// ... extract existing logic from runExperiment into here ...
	// Return the orchestrator and knowledge model
}
```

**Step 2: Wire suite to use orchestrator**

In `internal/cli/suite.go`, replace the "WOULD RUN" stub with actual orchestrator execution:

```go
if dryRun {
	fmt.Printf("VALID %s (%s)\n", exp.Metadata.Name, exp.Spec.Injection.Type)
	passed++
	continue
}

// Execute experiment
result, err := bundle.Orchestrator.Run(ctx, exp)
if err != nil {
	fmt.Printf("FAILED %s: %v\n", exp.Metadata.Name, err)
	failed++
	continue
}

if result.Verdict == v1alpha1.Resilient {
	passed++
} else {
	failed++
}
fmt.Printf("%s %s (verdict: %s)\n", result.Verdict, exp.Metadata.Name, result.Verdict)
```

**Step 3: Change dry-run default to false**

Change `--dry-run` default from `true` to `false` on the suite command, matching the `run` command behavior.

**Step 4: Add exit code for CI integration**

Return non-zero exit code when any experiment fails:

```go
if failed > 0 {
	return fmt.Errorf("%d experiment(s) failed", failed)
}
```

**Step 5: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... 2>&1 | tail -20`
Expected: All PASS

**Step 6: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add internal/cli/suite.go internal/cli/run.go
git commit -m "feat: make suite command execute experiments with CI exit codes"
```

---

### Task 22: Suite Parallel Execution

Add `--parallel` flag to the suite command for concurrent experiment execution.

**Files:**
- Modify: `internal/cli/suite.go`

**Step 1: Add parallel flag**

```go
cmd.Flags().IntVar(&parallel, "parallel", 1, "max concurrent experiments")
```

**Step 2: Implement parallel execution**

Use a worker pool pattern with a semaphore channel:

```go
type suiteResult struct {
	name    string
	verdict v1alpha1.Verdict
	err     error
}

sem := make(chan struct{}, parallel)
results := make(chan suiteResult, len(experimentFiles))
var wg sync.WaitGroup

for _, file := range experimentFiles {
	wg.Add(1)
	go func(f string) {
		defer wg.Done()
		sem <- struct{}{}
		defer func() { <-sem }()

		exp, err := experiment.Load(f)
		if err != nil {
			results <- suiteResult{name: f, err: err}
			return
		}

		result, err := bundle.Orchestrator.Run(ctx, exp)
		if err != nil {
			results <- suiteResult{name: exp.Metadata.Name, err: err}
			return
		}
		results <- suiteResult{name: exp.Metadata.Name, verdict: result.Verdict}
	}(file)
}

go func() {
	wg.Wait()
	close(results)
}()
```

**Step 3: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... 2>&1 | tail -20`
Expected: All PASS

**Step 4: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add internal/cli/suite.go
git commit -m "feat: add parallel execution support to suite command"
```

---

### Task 23: Suite JUnit Report Generation

Suite should generate a combined JUnit XML report for CI integration.

**Files:**
- Modify: `internal/cli/suite.go`
- Modify: `pkg/reporter/junit.go` (if needed)

**Step 1: Add report-dir flag to suite**

```go
cmd.Flags().StringVar(&reportDir, "report-dir", "", "directory for suite reports")
```

**Step 2: Generate suite-level report**

After all experiments complete, aggregate results into a JUnit-compatible XML report:

```go
if reportDir != "" {
	if err := os.MkdirAll(reportDir, 0750); err != nil {
		return fmt.Errorf("creating report directory: %w", err)
	}

	junitPath := fmt.Sprintf("%s/suite-%s.xml", reportDir, time.Now().Format("20060102-150405"))
	junitReporter, err := reporter.NewJUnitReporter(junitPath)
	if err != nil {
		return fmt.Errorf("creating JUnit reporter: %w", err)
	}
	defer junitReporter.Close()

	for _, r := range allResults {
		if r.Report != nil {
			junitReporter.Write(*r.Report)
		}
	}
}
```

**Step 3: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... 2>&1 | tail -20`
Expected: All PASS

**Step 4: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add internal/cli/suite.go
git commit -m "feat: add JUnit report generation to suite command for CI integration"
```

---

### Task 24: Define CheckType Constants

SteadyStateCheck.Type is a raw string. Define typed constants as flagged by the API review.

**Files:**
- Modify: `api/v1alpha1/types.go`
- Modify: `pkg/observer/kubernetes.go`
- Modify: `api/v1alpha1/types_test.go`

**Step 1: Write the test**

Add to `api/v1alpha1/types_test.go`:

```go
func TestCheckTypeConstants(t *testing.T) {
	types := []CheckType{CheckConditionTrue, CheckResourceExists, CheckPrometheusQuery}
	for _, ct := range types {
		assert.NotEmpty(t, string(ct))
	}
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL (undefined types)

**Step 3: Define CheckType**

Add to `api/v1alpha1/types.go`:

```go
// CheckType represents the type of steady-state check.
type CheckType string

const (
	CheckConditionTrue   CheckType = "conditionTrue"
	CheckResourceExists  CheckType = "resourceExists"
	CheckPrometheusQuery CheckType = "prometheusQuery"
)
```

Change `SteadyStateCheck.Type` from `string` to `CheckType`. Update `kubernetes.go` to use the constants in the switch statement.

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... 2>&1 | tail -20`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add api/v1alpha1/types.go pkg/observer/kubernetes.go api/v1alpha1/types_test.go
git commit -m "feat: define typed CheckType constants for steady-state checks"
```

---

### Task 25: Define DangerLevel Type

DangerLevel is a raw string. Define typed constants.

**Files:**
- Modify: `api/v1alpha1/types.go`
- Modify: `pkg/safety/blastradius.go`
- Modify: `api/v1alpha1/types_test.go`

**Step 1: Write the test**

Add to `api/v1alpha1/types_test.go`:

```go
func TestDangerLevelConstants(t *testing.T) {
	levels := []DangerLevel{DangerLevelLow, DangerLevelMedium, DangerLevelHigh}
	for _, dl := range levels {
		assert.NotEmpty(t, string(dl))
	}
}
```

**Step 2: Define DangerLevel type**

Add to `api/v1alpha1/types.go`:

```go
// DangerLevel represents the risk level of an injection.
type DangerLevel string

const (
	DangerLevelLow    DangerLevel = "low"
	DangerLevelMedium DangerLevel = "medium"
	DangerLevelHigh   DangerLevel = "high"
)
```

Change `InjectionSpec.DangerLevel` from `string` to `DangerLevel`. Update `blastradius.go` to use `v1alpha1.DangerLevelHigh`.

**Step 3: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... 2>&1 | tail -20`
Expected: All PASS

**Step 4: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add api/v1alpha1/types.go pkg/safety/blastradius.go api/v1alpha1/types_test.go
git commit -m "feat: define typed DangerLevel constants"
```

---

### Task 26: Admin Health Endpoint

Add `/chaos/health` endpoint for Kubernetes liveness/readiness probes.

**Files:**
- Modify: `pkg/sdk/admin.go`
- Modify: `pkg/sdk/admin_test.go`

**Step 1: Write the test**

Add to `pkg/sdk/admin_test.go`:

```go
func TestAdminHealthEndpoint(t *testing.T) {
	handler := NewAdminHandler(nil)

	req := httptest.NewRequest("GET", "/chaos/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "ok", body["status"])
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/ -run TestAdminHealthEndpoint -v`
Expected: FAIL (404)

**Step 3: Add health endpoint**

In `pkg/sdk/admin.go`, add to `NewAdminHandler`:

```go
mux.HandleFunc("/chaos/health", func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
})
```

**Step 4: Run all tests**

Run: `cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./pkg/sdk/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos
git add pkg/sdk/admin.go pkg/sdk/admin_test.go
git commit -m "feat: add /chaos/health endpoint for Kubernetes probes"
```

---

## Summary

| Part | Tasks | Focus |
|------|-------|-------|
| A | 1-14 | Critical Phase 2 review fixes (rollback persistence, lease expiry, race tests, structured logging, nil guards, clean expansion, test coverage, type safety, cleanup surfacing) |
| B | 15-20 | Advanced fault categories: memory, CPU, I/O, concurrency, network |
| C | 21-26 | Suite runner execution + parallel + JUnit, type safety (CheckType, DangerLevel), health endpoint |

**Total: 26 tasks**

**Deferred to Phase 4:**
- M14 (AI plugin: council framework, guardrails, hypothesis generation) — requires careful design of prompt engineering and model integration
- M17 (Controller mode CRD) — requires metav1.ObjectMeta migration, kubebuilder scaffolding, and reconciler implementation; better as a standalone phase

**After completing all tasks, run the full test suite:**

```bash
cd /Users/ugogiordano/workdir/rhoai/opendatahub-io/odh-platform-chaos && go test ./... -v
```
