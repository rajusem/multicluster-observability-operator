## Fix Plan for OBSINTA-1313

### Version
Plan v1 | Audit skipped — simple fix (2 files, <20 lines), all confidence HIGH, regression signal with clear root cause

### Root Cause
`GetDefaultRSPlacement()` in `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` (line 20-36) creates a `Placement` resource with an **empty predicates list** (`Predicates: []clusterv1beta1.ClusterPredicate{}`). In the Open Cluster Management (OCM) Placement API, an empty predicates list matches ALL managed clusters — including non-OpenShift clusters like AKS, EKS, and GKE.

The `rs-prom-rules-policy` deploys a `PrometheusRule` resource into the `openshift-monitoring` namespace via a `ConfigurationPolicy`. This namespace only exists on OpenShift clusters. When these policies are deployed to non-OpenShift clusters (e.g. AKS), the policy fails because the target namespace doesn't exist — this causes the `openshift-monitoring` missing error noted in the ticket.

The branch `test-autofix-rs-placement-v17` is based on `main` at commit `877d167b`, which predates the fix that was later merged into main (commit `b3fa6f20` — "Restrict right-sizing policies to OpenShift clusters only", by Darshan Vandra). This fix needs to be applied to this branch.

### Approach
Add a `vendor=OpenShift` label selector predicate using `MatchExpressions` with `LabelSelectorOpIn` to the default placement configuration in `GetDefaultRSPlacement()`. This ensures the placement only targets OpenShift clusters where the `openshift-monitoring` namespace exists.

The fix mirrors the exact change from commit `b3fa6f20` on `main` (reviewed and approved). Both `rs-namespace` and `rs-virtualization` components share `GetDefaultRSPlacement()`, so this single-function change fixes both policies.

### Planned Files
- `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` — Replace `Predicates: []clusterv1beta1.ClusterPredicate{}` with a `vendor=OpenShift` predicate using `MatchExpressions`; add doc comment explaining the OpenShift-only targeting
- `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go` — Update `TestGetDefaultRSPlacement` to assert 1 predicate with correct `MatchExpressions` (vendor=OpenShift) instead of asserting empty predicates

### Exact Code Changes

#### `placement.go` — Replace the `GetDefaultRSPlacement` function body

Replace:
```go
// GetDefaultRSPlacement creates a default placement configuration for right-sizing
func GetDefaultRSPlacement() clusterv1beta1.Placement {
	return clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{},
```

With:
```go
// GetDefaultRSPlacement creates a default placement configuration for right-sizing.
// Only OpenShift clusters are targeted because the policies enforce PrometheusRules
// in openshift-monitoring, which does not exist on non-OpenShift distributions (e.g. AKS).
func GetDefaultRSPlacement() clusterv1beta1.Placement {
	return clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{
				{
					RequiredClusterSelector: clusterv1beta1.ClusterSelector{
						LabelSelector: metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "vendor",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"OpenShift"},
								},
							},
						},
					},
				},
			},
```

Note: `metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"` is already imported in the file (used in `CreateUpdateRSPlacement`), so no new import is needed.

#### `placement_test.go` — Update `TestGetDefaultRSPlacement`

Replace the line:
```go
assert.Empty(t, placement.Spec.Predicates)
```

With:
```go
assert.Len(t, placement.Spec.Predicates, 1)

// Verify the vendor=OpenShift cluster selector
predicate := placement.Spec.Predicates[0]
matchExprs := predicate.RequiredClusterSelector.LabelSelector.MatchExpressions
assert.Len(t, matchExprs, 1)
assert.Equal(t, "vendor", matchExprs[0].Key)
assert.Equal(t, metav1.LabelSelectorOpIn, matchExprs[0].Operator)
assert.Equal(t, []string{"OpenShift"}, matchExprs[0].Values)
```

Add `metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"` to the test file imports (already present in the test file on this branch).

### Dependencies & Side Effects
- **No public API change** — internal default configuration
- **ConfigMap impact** — The default ConfigMap YAML for `placementConfiguration` will now include the predicate. Existing ConfigMaps already deployed are NOT retroactively updated; the change only affects new deployments or when the ConfigMap is recreated.
- **No downstream consumer impact** — Users with custom placements are unaffected. Users relying on the default placement targeting all clusters were never functional on non-OpenShift anyway.
- **Both components fixed** — rs-namespace AND rs-virtualization both call `GetDefaultRSPlacement()`.

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Existing OpenShift clusters break | LOW | HIGH | Only default predicate changes; custom placements unaffected |
| Wrong label value | LOW | MEDIUM | `vendor=OpenShift` matches what OpenShift clusters have; consistent with placementrule_controller.go and main branch fix |
| ConfigMap not updated retroactively | CERTAIN | LOW | Intentional — existing deployments are preserved |

### Test Strategy
- Run `go test ./operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/...` to verify all placement tests pass
- Updated test `TestGetDefaultRSPlacement` verifies the new predicate is set correctly
- Existing tests `TestCreateUpdatePlacement_CreatesNew` and `TestCreateUpdatePlacement_UpdatesExisting` remain unchanged

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | `GetDefaultRSPlacement()` placement.go:20-36 creates empty predicates; this exact bug was fixed in main (commit b3fa6f20) |
| Approach correctness | HIGH | Fix mirrors already-reviewed-and-merged main branch change (b3fa6f20) |
| Scope completeness | HIGH | Single function change affects both rs-namespace and rs-virtualization; only test file needs corresponding update |

### Audit Trail
- Audit skipped per complexity gate Rule 5: simple fix (2 files, <20 lines), all confidence HIGH, regression signal with clear root cause
- Prior fix in main: commit `b3fa6f20` authored by Darshan Vandra, merged May 6 2026
