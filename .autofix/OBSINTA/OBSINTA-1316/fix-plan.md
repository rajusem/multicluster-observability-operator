# Fix Plan for OBSINTA-1316

## Version
Plan v1 | Iteration 0 (initial draft)

### Root Cause
The `GetDefaultRSPlacement()` function in `rs-utility/placement.go` creates a KDM (Kapitan Deployment Manager) Placement resource with an **empty `Predicates` list** (`Predicates: []clusterv1beta1.ClusterPredicate{}`). An empty predicates list in a Placement selects ALL managed clusters, without filtering by cluster vendor. This causes the `rs-prom-rules-policy` (a ConfigurationPolicy deploying a PrometheusRule) to be deployed to **all** managed clusters (including AKS, GKE, etc.) instead of only OpenShift clusters.

The ticket description states: *"rs-prom-rules-policy deployed to all clusters including AKS. openshift-monitoring namespace missing on non-OpenShift."* The non-OpenShift clusters (e.g., AKS) don't have the `openshift-monitoring` namespace, which can cause policy compliance issues.

Evidence:
- **File**: `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go:23`
- **Code**: `Predicates: []clusterv1beta1.ClusterPredicate{}` (empty)
- **Related**: The `placementrule_controller.go:808` shows the correct pattern — checking `vendor == "OpenShift"` for cluster filtering
- **Test**: `rs-utility/placement_test.go:22` asserts `assert.Empty(t, placement.Spec.Predicates)`, masking the bug

### Approach
Add a `ClusterPredicate` to the default Placement in `GetDefaultRSPlacement()` that filters clusters by the `vendor=OpenShift` label. This uses the `RequiredClusterSelector` field with a `LabelSelector` containing `MatchLabels: {"vendor": "OpenShift"}`.

This approach:
1. Is the canonical way to filter clusters in KDM Placement resources
2. Uses the `vendor` label which is already set on all managed cluster CRs (as shown in `placementrule_controller.go`)
3. Is a minimal, targeted change (1 field) that doesn't affect any other logic
4. Aligns with existing patterns in other MCO controllers

### Alternatives Considered

| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Add predicate in `placementbinding.go` | Could filter at binding level | Binding doesn't support label filtering; this is a Placement concern | Wrong API layer |
| 2 | Use `matchExpressions` for more complex filtering | More flexible | Overly complex for a simple equality check on `vendor` | Not needed |
| 3 | Set vendor filter in the PrometheusRule policy itself | Could filter at policy level | Policy level filtering doesn't prevent the Policy object from being created on non-OpenShift clusters | Doesn't solve the root problem |
| 4 | Add vendor check in Go code before creating placement | Early validation | Would require changes to configmap handling and Go code paths | Doesn't use KDM's native capability |


### Files to Change

| File | Change | Reason |
|------|--------|--------|
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` | Add a `ClusterPredicate` with `vendor=OpenShift` label selector in `GetDefaultRSPlacement()` | Fix: filter placement to OpenShift clusters only |
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go` | Update `TestGetDefaultRSPlacement` to verify the predicate contains a `vendor=OpenShift` match label | Test coverage: verify the predicate exists |

### Dependencies & Side Effects
- [ ] Public API change? **Narrowly scoped** — changes the default Placement spec but not the MCO CR API
- [ ] Config / env var change? **No** — label filtering is inline in Go code
- [ ] Database migration? **No**
- [ ] Downstream consumer impact? **Yes** — the `rs-prom-rules-policy` placement will now only select OpenShift-managed clusters; existing non-OpenShift clusters will stop receiving the PrometheusRule (expected behavior)
- [ ] Error handling / logging change? **No**
- [ ] Performance characteristics change? **No** — KDM evaluates label selectors at runtime, same cost

### Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Removing policy from already-selected non-OpenShift clusters | Medium | Low | Intentional behavior; these clusters can't use the policy anyway (missing namespace) |
| Breaking future non-OpenShift analytics features | Low | Medium | The filter is vendor-specific; future features should use appropriate filters |
| Test regression (assertion change) | Low | Low | Update the assertion as part of this change |

### Test Strategy
- **Existing tests to verify**: `TestGetDefaultRSPlacement` (placement_test.go), `TestGetDefaultRSPlacement_UsesCorrectConstants` (rs-namespace/placement_test.go)
- **New regression test**: Update `TestGetDefaultRSPlacement` to assert the predicate contains a `vendor=OpenShift` MatchLabel requirement. Optionally add an integration test verifying the Placement YAML output includes the predicate.
- **E2E test**: The referenced `tests/pkg/tests/observability_right_sizing_test.go` tests the `rs-prom-rules-policy` existence; it should continue to pass on OpenShift clusters but won't test non-OpenShift behavior (which is now expected to not have the policy).

### Confidence

| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Direct code inspection — `Predicates: []clusterv1beta1.ClusterPredicate{}` is empty at `placement.go:23`; test at `placement_test.go:22` asserts emptiness, confirming this is the intended (buggy) code path |
| Approach correctness | HIGH | KDM Placement API supports `ClusterPredicate` with `RequiredClusterSelector` + `LabelSelector` (already used in `helper_test.go:42-47`); `vendor=OpenShift` label exists on all OCM managed clusters |
| Scope completeness | HIGH | Only two files need changes: the source function and its test. No cross-cutting changes needed |

### Investigation Strategy

**Signals detected**: environment (behavior differs by cluster vendor/type)
**Strategy used**: Standard investigation
**Key findings from strategy**:
- The `GetDefaultRSPlacement()` in `rs-utility/placement.go:23` has empty predicates → selects ALL clusters
- KDM's Placement API supports cluster filtering via ClusterPredicate (used in `helper_test.go`)
- The existing test `placement_test.go:22` asserts empty predicates, confirming the bug was known-but-accepted or never questioned
- Other parts of the codebase (e.g., `placementrule_controller.go:808`) already check `vendor == "OpenShift"` for cluster-specific logic
