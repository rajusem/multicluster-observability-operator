# Fix Plan for OBSINTA-1320

### Version
Plan v1 | Iteration 0 (initial draft)

### Root Cause
The `GetDefaultRSPlacement()` function in `rs-utility/placement.go` creates a Placement resource with **empty `Predicates`** (`[]clusterv1beta1.ClusterPredicate{}`). Since OCM evaluates all clusters against the empty predicate list, all managed clusters — including non-OpenShift ones like AKS — match and receive the `rs-prom-rules-policy`. The policy (and its associated PrometheusRules) are thus deployed to non-OpenShift clusters where they are unnecessary.

The expected behavior is to only deploy right-sizing Prometheus rules to OpenShift clusters, matching the same `vendor=OpenShift` selector pattern already used elsewhere in `main.go` for ManagedCluster caching (`vendor!=auto-detect,observability!=disabled`).

### Approach
Add a `ClusterPredicate` with a `LabelSelector` containing `MatchLabels: {"vendor": "OpenShift"}` to the placement created by `GetDefaultRSPlacement()`. This ensures OCM only places the policy on OpenShift-managed clusters.

This approach:
- Uses the standard OCM `ClusterPredicate` API (already imported and used in tests)
- Matches the existing pattern in `main.go` line 193 (`vendor!=auto-detect`)
- Is additive — does not change the cleanup path or existing tolerations
- Affects only the right-sizing placement; other operators' placements are untouched

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Add placement predicate with vendor label filter | Minimal change, standard OCM pattern | — | **Selected** |
| 2 | Use ResourceFilter in ConfigurationPolicy for non-OpenShift exclusion | Works at policy application level | Vendor label may not exist on all non-OCP clusters; doesn't address Placement itself | Placement-level filtering is the correct layer |
| 3 | Introduce a configurable platform filter in the ConfigMap | Flexible for future use cases | Over-engineers a simple fix; adds ConfigMap migration risk | Not needed for this issue |

### Files to Change
| File | Change | Reason |
|------|--------|--------|
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` | Add `ClusterPredicate` with `LabelSelector.MatchLabels: {"vendor": "OpenShift"}` to `GetDefaultRSPlacement()` | Core fix — restricts placement to OpenShift clusters |
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go` | Update `TestGetDefaultRSPlacement` expectation — predicates no longer empty | Test must reflect new expected behavior |

### Dependencies & Side Effects
- [x] Public API change? No — internal function only
- [ ] Config / env var change? No
- [ ] Database migration? N/A
- [ ] Downstream consumer impact? The policy won't be placed on non-OpenShift clusters (expected behavior)
- [x] Error handling / logging change? No
- [x] Performance characteristics change? Negligible — one extra label check per cluster

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Vendor label not present on OpenShift managed clusters | Low | Policy not placed on any cluster | `vendor=OpenShift` is standard on ACM-managed OpenShift clusters (used elsewhere in `main.go`) |
| Existing deployments lose placement on already-created clusters | Low | Medium | OCM replaces Placement spec on each reconcile; updated predicated will be applied |

### Test Strategy
- Existing unit test `TestGetDefaultRSPlacement` needs updating — check predicates are no longer empty
- `TestFormatYAML_WithPlacement` in `helper_test.go` already validates the predicate YAML structure works
- `TestCreateUpdatePlacement_CreatesNew` should continue to pass (it passes its own placement spec)

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Empty predicates in `GetDefaultRSPlacement()` directly cause wildcard cluster selection |
| Approach correctness | HIGH | Uses `ClusterPredicate` + `LabelSelector` pattern validated by `helper_test.go:42-47` |
| Scope completeness | HIGH | Single-function change covers both namespace and virtualization right-sizing |

### Investigation Strategy
**Signals detected**: default
**Strategy used**: Standard investigation (grep, file reads, code path tracing)
**Key findings from strategy**:
- `GetDefaultRSPlacement()` creates placement with empty `Predicates: []clusterv1beta1.ClusterPredicate{}` (line 23-23)
- Same function used by both namespace and virtualization components via `GetDefaultRSPlacement()`
- No `vendor` label filtering exists in any rightsizing code (confirmed: zero matches)
- `main.go:193` shows the standard `vendor` filtering pattern: `"vendor!=auto-detect,observability!=disabled"`
- OCM `ClusterPredicate` with `LabelSelector.MatchLabels` is the correct abstraction for cluster-level filtering
