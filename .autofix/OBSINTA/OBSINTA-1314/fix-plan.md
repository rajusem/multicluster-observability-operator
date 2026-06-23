# Fix Plan for OBSINTA-1314

### Version
Plan v1 | Iteration 0 (initial draft) — APPROVED (audit skipped: simple fix, HIGH confidence, environment signal)

### Root Cause
The `GetDefaultRSPlacement()` function in `rs-utility/placement.go` (line 20-36) returns a Placement resource with **empty predicates** (`Predicates: []clusterv1beta1.ClusterPredicate{}`). This causes the ACM Placement controller to match **all** managed clusters in the fleet — including non-OpenShift clusters like AKS — instead of filtering for `vendor=OpenShift` only.

Both the namespace right-sizing (`rs-namespace/configmap.go:25`) and virtualization right-sizing (`rs-virtualization/configmap.go:24`) components call `GetDefaultRSPlacement()`, so both `rs-prom-rules-policy` variants are deployed to all clusters.

### Approach
Add a `ClusterPredicate` with a `RequiredClusterSelector` LabelSelector matching `vendor: OpenShift` to the default placement configuration. This ensures right-sizing Prometheus rule policies are only applied to OpenShift-managed clusters.

The `RequiredClusterSelector` field (not `ClusterNameSelector`) is the correct ACM Placement API field, as confirmed by the existing test in `helper_test.go:40-50`.

### Why this approach
- **Minimal scope**: Only the default placement function needs to change. The `CreateUpdateRSPlacement` function propagates predicates already, so no changes needed there.
- **Follows established pattern**: The `helper_test.go` test shows the exact API structure used elsewhere in the codebase.
- **Non-breaking**: Existing OpenShift clusters are labeled `vendor: OpenShift`, so this adds filtering without changing existing behavior for OpenShift.

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Add cluster label selector in ConfigMap | Configurable per-cluster | Requires users to update ConfigMap; less robust default | Default should be correct without user intervention |
| 2 | Add vendor check in controller reconciliation | Flexible | More complex, error-prone, distributed logic | Placement-level filtering is the correct ACM-native approach |
| 3 | Use ComponentClusterLabelSelector in Predicates | Flexible multi-select | Overly complex for single-label filter | `RequiredClusterSelector` with `MatchLabels` is simpler and correct |

### Files to Change
| File | Change | Reason |
|------|--------|--------|
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` | Add `ClusterPredicate` with `RequiredClusterSelector` LabelSelector `{"vendor": "OpenShift"}` to `GetDefaultRSPlacement()` | Root cause fix — restrict placement to OpenShift clusters only |
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go` | Update `TestGetDefaultRSPlacement` to assert predicate exists with `vendor: OpenShift` label | Test must verify new predicate, not assert empty predicates |

### Dependencies & Side Effects
- [ ] Public API change? No — Placement spec is internal to the operator
- [ ] Config / env var change? No
- [ ] Database migration? No
- [ ] Downstream consumer impact? No — policies will simply not deploy to non-OpenShift clusters (correct behavior)
- [ ] Error handling / logging change? No
- [ ] Performance characteristics change? No — label selector is evaluated by ACM controller, same cost as before

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Non-OpenShift clusters that need RS policies won't receive them | LOW | Medium (if users have custom non-OpenShift clusters that need RS) | Existing behavior is already incorrect; this fix aligns with expected behavior documented in ticket |
| Placement update causes policy churn | MEDIUM | LOW (transient) | ACM Placement controller handles updates gracefully; existing tolerations prevent flapping |

### Test Strategy
- **Existing tests to verify**: `TestGetDefaultRSPlacement` must pass with new predicate assertion
- **New logic verified by**: Updated test that asserts the `vendor: OpenShift` predicate exists and `RequiredClusterSelector` LabelSelector contains the correct match labels
- **Unit tests**: Will need to update `TestGetDefaultRSPlacement` (line 19-31) — currently asserts empty predicates, now must verify the vendor label predicate

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Direct code verification — `Predicates: []clusterv1beta1.ClusterPredicate{}` at placement.go:23 with no predicate added anywhere |
| Approach correctness | HIGH | `helper_test.go:40-50` shows the established pattern using `RequiredClusterSelector` with `LabelSelector` |
| Scope completeness | HIGH | Only `GetDefaultRSPlacement()` needs to change; callers pass the struct through unchanged |

### Investigation Strategy
**Signals detected**: environment
**Strategy used**: Standard investigation — code path tracing from Deployment → ConfigMap → Placement → Policy
**Key findings from strategy**:
- `GetDefaultRSPlacement()` returns empty predicates (line 23)
- Both `rs-namespace/configmap.go:25` and `rs-virtualization/configmap.go:24` use this function
- `CreateUpdateRSPlacement` (line 58, 77) directly assigns `placementConfig.Spec` — predicates are propagated as-is
- Existing test `TestFormatYAML_WithPlacement` (helper_test.go) demonstrates the correct ACM Placement API structure with `RequiredClusterSelector`

### Audit Trail
- **Audit**: Skipped — Simple fix (2 files, ~5 lines), all confidence HIGH, environment signal with clear root cause
- **Rules applied**: Complexity Gate Rule 5 (all simple, all HIGH confidence, environment signal → skip Phase 4B)
