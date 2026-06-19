## Fix Plan for OBSINTA-1300

### Version
Plan v1 | Audit skipped — simple fix, all confidence HIGH, ≤2 files

### Root Cause
The `GetDefaultRSPlacement()` function in `rs-utility/placement.go` (line 20-36) creates a `Placement` resource with an **empty predicates list** (`Predicates: []clusterv1beta1.ClusterPredicate{}`). In the Open Cluster Management (OCM) Placement API, an empty predicates list matches ALL managed clusters — including non-OpenShift clusters like AKS, EKS, and GKE.

The right-sizing policies (`rs-prom-rules-policy` for namespace and `rs-virt-prom-rules-policy` for virtualization) deploy `PrometheusRule` resources into the `openshift-monitoring` namespace. This namespace only exists on OpenShift clusters. When these policies are deployed to non-OpenShift clusters, they fail because the target namespace doesn't exist.

### Approach
Add a `vendor=OpenShift` label selector predicate to the default placement configuration in `GetDefaultRSPlacement()`. This ensures the placement only targets OpenShift clusters where the `openshift-monitoring` namespace exists.

The change uses the OCM Placement API's `ClusterPredicate` with `RequiredClusterSelector` and `LabelSelector` — the standard mechanism for cluster filtering. This same pattern is already used in the codebase's test fixtures (`rs-utility/helper_test.go` lines 34-50).

Both `rs-namespace` and `rs-virtualization` components call `GetDefaultRSPlacement()` as their default, so this single change fixes both components.

**Specific code change in `GetDefaultRSPlacement()`:**

Replace:
```go
Predicates: []clusterv1beta1.ClusterPredicate{},
```

With:
```go
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
```

This adds `metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"` to the imports (already used in other files in the same package).

### Planned Files
- `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` — Add `vendor=OpenShift` label selector predicate to `GetDefaultRSPlacement()` and add `metav1` import
- `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go` — Update `TestGetDefaultRSPlacement` to verify the `vendor=OpenShift` predicate is present instead of asserting empty predicates

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Add predicate in each component's `configmap.go` | Could allow different defaults per component | Duplicates logic, both need the same filter | Both components have identical requirements |
| 2 | Add namespace existence check in the policy template | Handles it at apply time | Wrong layer — the policy shouldn't be deployed at all | Wastes resources and creates noisy errors |
| 3 | Use `clusterSets` in Placement spec | Could target specific cluster sets | Requires pre-existing ClusterSet configuration | Label selector is simpler and more portable |

### Dependencies & Side Effects
- **No public API change** — this is an internal default configuration
- **ConfigMap impact** — The default ConfigMap YAML for `placementConfiguration` will now include the predicate. Existing ConfigMaps already deployed are NOT retroactively updated; the change only affects new deployments or when the ConfigMap is recreated.
- **No downstream consumer impact** — Users who customized their placement are unaffected. Users relying on the default placement targeting all clusters were never functional on non-OpenShift anyway.

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Existing deployments unaffected | HIGH | LOW | Change only affects new/recreated ConfigMaps |
| Break custom placements | LOW | LOW | Custom placements override the default |
| Wrong label value | LOW | MEDIUM | Using exact `vendor=OpenShift` matching, consistent with `placementrule_controller.go:808` |

### Test Strategy
- **Update existing test**: `TestGetDefaultRSPlacement` — change assertion from `assert.Empty(t, placement.Spec.Predicates)` to verify predicates contain `vendor=OpenShift` label selector
- **Existing tests unaffected**: `TestCreateUpdatePlacement_CreatesNew`, `TestCreateUpdatePlacement_UpdatesExisting` — these use custom specs, not the defaults

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | `GetDefaultRSPlacement()` in placement.go:20-36 creates empty predicates; both rs-namespace and rs-virtualization use this via configmap.go |
| Approach correctness | HIGH | OCM Placement API predicates with label selectors are the standard mechanism; existing test in helper_test.go:34-50 shows this exact pattern |
| Scope completeness | HIGH | Single function change affects both rs-namespace and rs-virtualization; only test file needs corresponding update |

### Audit Trail
- Audit skipped per complexity gate Rule 5: simple fix (2 files, <20 lines), all confidence HIGH, environment signal
