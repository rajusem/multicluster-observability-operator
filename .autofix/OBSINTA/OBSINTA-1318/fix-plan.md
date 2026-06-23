# Fix Plan — OBSINTA-1318: Right-sizing non-OpenShift

## Ticket
[OBSINTA-1318](https://stage-redhat.atlassian.net/browse/OBSINTA-1318)

## Root Cause
The `GetDefaultRSPlacement()` function in `rs-utility/placement.go` creates a `clusterv1beta1.Placement` with **empty `Predicates`**. A Placement with no Predicates applies to **all managed clusters** (including non-OpenShift clusters like AKS).

This means the `rs-prom-rules-policy` (right-sizing Prometheus rules) is deployed to every managed cluster in the fleet, regardless of cluster vendor. The expected behavior is that this policy should only target OpenShift clusters (`vendor=OpenShift`), as the right-sizing namespace rules query kube-state-metrics metrics which are only meaningful on OpenShift.

The existing `GetDefaultRSPlacement()` tolerations (unreachable/unavailable) correctly allow the policy to reach all clusters regardless of connectivity, but the missing Predicate means there is no vendor-based filter.

## Impact
- `rs-prom-rules-policy` is applied on ALL clusters including AKS and other non-OpenShift clusters
- `openshift-monitoring` namespace is missing from non-OpenShift clusters (expected), but the policy still fires rules there
- Expected: `vendor=OpenShift` only

## Proposed Fix

### File: `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go`

**Change `GetDefaultRSPlacement()` to add a `ClusterPredicate` that filters for OpenShift clusters:**

```go
func GetDefaultRSPlacement() clusterv1beta1.Placement {
	return clusterv1beta1.Placement{
		Spec: clusterv1beta1.PlacementSpec{
			Predicates: []clusterv1beta1.ClusterPredicate{
				{
					RequiredClusterSelector: clusterv1beta1.ClusterSelector{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"vendor": "OpenShift",
							},
						},
					},
				},
			},
			Tolerations: []clusterv1beta1.Toleration{
				{
					Key:      "cluster.open-cluster-management.io/unreachable",
					Operator: clusterv1beta1.TolerationOpExists,
				},
				{
					Key:      "cluster.open-cluster-management.io/unavailable",
					Operator: clusterv1beta1.TolerationOpExists,
				},
			},
		},
	}
}
```

### File: `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go`

**Update `TestGetDefaultRSPlacement` to expect the new Predicate:**

Change:
```go
assert.Empty(t, placement.Spec.Predicates)
```
To:
```go
assert.Len(t, placement.Spec.Predicates, 1)
assert.Len(t, placement.Spec.Predicates[0].RequiredClusterSelector.LabelSelector.MatchLabels, 1)
assert.Equal(t, "OpenShift", placement.Spec.Predicates[0].RequiredClusterSelector.LabelSelector.MatchLabels["vendor"])
```

### File: `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/helper_test.go`

**The test `TestFormatYAML_WithPlacement` already uses a Placement with a Predicate (test data includes `MatchLabels: environment: prod`).** This test constructs its own Placement for testing `FormatYAML` serialization and does not call `GetDefaultRSPlacement()`. It will continue to work as-is since it's testing YAML formatting, not the specific output of `GetDefaultRSPlacement()`.

## Files Changed
| File | Change |
|------|--------|
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` | Add `ClusterPredicate` with `vendor=OpenShift` label selector to `GetDefaultRSPlacement()` |
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go` | Update test expectation from empty Predicates to 1 Predicate with `vendor: OpenShift` |

## Risk Assessment
- **Low risk** — This is a declarative label selector on a Placement resource. Changing from "match all" to "match OpenShift only" is a scope reduction, not a behavioral regression.
- On AKS and other non-OpenShift managed clusters, the `rs-prom-rules-policy` will no longer be deployed, which is the **expected** behavior.
- Existing OpenShift clusters will continue to receive the policy unchanged.

## Validation
After deployment:
1. Verify `rs-prom-rules-policy` exists only in OpenShift cluster namespaces
2. Verify it does NOT exist on AKS or other non-OpenShift managed cluster namespaces
3. Verify Prometheus rules are only active on OpenShift clusters (checking `openshift-monitoring` target)
