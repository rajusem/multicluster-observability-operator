## Fix Plan for OBSINTA-1364

### Version
Plan v1 | Iteration 0 (initial draft) | Audit skipped (simple fix, all confidence HIGH, 2 files, <20 lines)

### Root Cause
`GetDefaultRSPlacement()` in `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` (line 20-36) creates a Placement resource with empty `Predicates: []clusterv1beta1.ClusterPredicate{}`. An empty Predicates list means the Placement matches ALL managed clusters — including non-OpenShift clusters (EKS, GKE, AKS, etc.) that lack the `openshift-monitoring` namespace. The right-sizing policy (`rs-prom-rules-policy`) wraps a `PrometheusRule` targeting `openshift-monitoring`, so deploying it to non-OpenShift clusters fails because the namespace doesn't exist.

### Approach
Add a `ClusterPredicate` with `RequiredClusterSelector` using `MatchLabels: {"vendor": "OpenShift"}` to the default Placement's `Predicates` field. This restricts the Placement to only match OpenShift clusters, which is the only platform where `openshift-monitoring` exists. This pattern is already established in the codebase — the `placementrule_controller.go` uses `vendor: OpenShift` label matching for similar purposes (line 808), and `main.go` uses `vendor!=auto-detect` in its LabelSelector (line 193).

Both `rs-namespace` and `rs-virtualization` components call `GetDefaultRSPlacement()`, so the single fix propagates to both right-sizing features.

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Add namespace existence check in policy template | Handles edge cases | More complex, runtime check on each cluster, doesn't prevent unnecessary policy propagation | Over-engineered; Placement filtering is the OCM-idiomatic way |
| 2 | Add predicate at each call site instead of default | More explicit | Duplicates logic in both rs-namespace and rs-virtualization | Violates DRY; default should be safe by default |
| 3 | Use ClaimSelector instead of LabelSelector | Another OCM API | Less commonly used in this codebase, vendor label is the standard pattern | LabelSelector with vendor:OpenShift is the established pattern |

### Files to Change
| File | Change | Reason |
|------|--------|--------|
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` | Add `ClusterPredicate` with `RequiredClusterSelector` matching `vendor: OpenShift` to the `Predicates` field in `GetDefaultRSPlacement()` | Filter Placement to only OpenShift clusters |
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go` | Update `TestGetDefaultRSPlacement` to assert Predicates contains the vendor:OpenShift selector instead of being empty | Verify the fix is correct |

### Dependencies & Side Effects
- [ ] Public API change? **No** — internal default configuration only
- [ ] Config / env var change? **No**
- [ ] Database migration? **No**
- [ ] Downstream consumer impact? **No** — users who have already customized their Placement via the ConfigMap are unaffected (the default is only used on initial creation)
- [ ] Error handling / logging change? **No**
- [ ] Performance characteristics change? **No** — Placement filtering is a standard OCM operation

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Existing clusters with customized Placement are overwritten | LOW | LOW | `CreateUpdateRSPlacement` only creates if not found; existing customized Placements are preserved unless the ConfigMap triggers an update |
| Non-OpenShift clusters intentionally running openshift-monitoring | VERY LOW | LOW | Edge case; users can customize via ConfigMap's placementConfiguration to override the default |

### Test Strategy
- Existing tests to verify: `TestGetDefaultRSPlacement`, `TestCreateUpdatePlacement_CreatesNew`, `TestCreateUpdatePlacement_UpdatesExisting`, `TestFormatYAML_WithPlacement`
- Updated test: `TestGetDefaultRSPlacement` — assert Predicates contains one entry with vendor:OpenShift MatchLabels
- Run: `make unit-tests` or `go test ./operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/...`

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Empty `Predicates` at placement.go:23 directly causes all-cluster matching. The `openshift-monitoring` namespace constant at types.go:17 confirms OpenShift dependency. |
| Approach correctness | HIGH | `vendor: OpenShift` label matching is the established codebase pattern (placementrule_controller.go:808, main.go:193). The `ClusterPredicate` type is already used in helper_test.go:42-47. |
| Scope completeness | HIGH | `GetDefaultRSPlacement()` is the single source of default Placement config, called by both rs-namespace and rs-virtualization components. One change fixes both. |

### Investigation Strategy
**Signals detected**: environment (non-OpenShift clusters missing openshift-monitoring namespace)
**Strategy used**: Standard investigation (grep for rs-prom-rules-policy, trace Placement creation path)
**Key findings from strategy**:
  - The `GetDefaultRSPlacement()` function is the sole source of default Placement configuration
  - Both rs-namespace and rs-virtualization call this function via their respective `GetDefaultRS*Config()` functions
  - The codebase already uses `vendor: OpenShift` label matching for similar cluster filtering purposes
  - The OCM `ClusterPredicate` with `RequiredClusterSelector` and `LabelSelector` is the idiomatic way to filter Placements
