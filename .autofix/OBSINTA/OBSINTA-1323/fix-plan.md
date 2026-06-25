# Fix Plan for OBSINTA-1323

### Version
Plan v1 | Iteration 0 (initial draft — audit skipped: simple fix, all confidence HIGH)

### Root Cause
The `GetDefaultRSPlacement()` function in `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` creates a Placement resource with empty `Predicates` (`Predicates: []clusterv1beta1.ClusterPredicate{}`). This means the `rs-prom-rules-policy` is applied to ALL managed clusters, including non-OpenShift clusters like AKS. The Prometheus rules in this policy are OpenShift-specific and should only be deployed to OpenShift clusters.

The expected behavior is that the placement should filter clusters by `vendor: OpenShift` so non-OpenShift clusters (AKS, etc.) are excluded from receiving the policy.

Reference: [clusterv1beta1 docs](https://pkg.go.dev/open-cluster-management.io/api/cluster/v1beta1#ClusterPredicate) — `ClusterPredicate` with `RequiredClusterSelector` and `LabelSelector` with `MatchLabels` is the standard way to filter clusters by label in OCM.

### Approach
Set the default Predicates in `GetDefaultRSPlacement()` to include a `ClusterPredicate` that selects only clusters with `vendor: OpenShift`.

**Why this approach:**
- One-line change to the existing `GetDefaultRSPlacement()` function
- Uses the standard OCM `ClusterPredicate` pattern already documented in the project
- The `ClusterPredicate.RequiredClusterSelector.LabelSelector.MatchLabels` structure is already imported and usable (the types `clusterv1beta1.ClusterPredicate`, `clusterv1beta1.ClusterSelector`, and `metav1.LabelSelector` are available)
- The same fix applies to both namespace and virtualization right-sizing components since they both call `GetDefaultRSPlacement()`
- Existing test `TestGetDefaultRSPlacement` in `placement_test.go` asserts `Predicates` is empty — this test needs updating

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Add ClusterPredicate with vendor=OpenShift | Minimal change, single location | — | Selected |
| 2 | Add a config option to enable/disable vendor filtering | Flexible | Adds config complexity; vendor=OpenShift should always be the default for RS policies | Over-engineering for a static correctness fix |
| 3 | Add NamespaceBinding to filter to openshift-monitoring only | Simple | Won't help — the namespace filter is for Prometheus rule targets, not cluster selection. The Placement itself targets the wrong clusters | Wrong layer of filtering |

### Files to Change
| File | Change | Reason |
|------|--------|--------|
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` | Add `Predicates` with `ClusterPredicate{RequiredClusterSelector: {LabelSelector: {MatchLabels: {"vendor": "OpenShift"}}}}` to `GetDefaultRSPlacement()` | Fixes the root cause: limits placement to OpenShift clusters only |
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go` | Update `TestGetDefaultRSPlacement` assertion from `assert.Empty(t, placement.Spec.Predicates)` to verify the ClusterPredicate with vendor=OpenShift is present | Test must match new expected behavior |

### Dependencies & Side Effects
- [x] Public API change? No — internal function, no external API
- [ ] Config / env var change? No
- [ ] Database migration? N/A
- [ ] Downstream consumer impact? Yes — `rs-virtualization/placement.go` also uses `rsutility.GetDefaultRSPlacement()` via the component flow and will benefit from the same fix
- [ ] Error handling / logging change? No
- [ ] Performance characteristics change? Negligible — one extra label selector during placement evaluation

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| AKS/non-OpenShift clusters lose access to Prometheus rules | MEDIUM | LOW — they were never intended to receive OpenShift-specific rules | The filter ensures OpenShift clusters get the rules; AKS clusters were getting OpenShift rules incorrectly |
| Existing MCO CRs with custom configs are affected | LOW | LOW — only affects default placement. Custom placement configs in ConfigMaps are unaffected | The change only modifies the default config; manual ConfigMap edits override this |
| Test breakage | HIGH | LOW — `TestGetDefaultRSPlacement` expects empty predicates; test updated as part of fix | Plan includes test update |

### Test Strategy
- **Existing tests to verify**: 
  - `TestGetDefaultRSPlacement` in `placement_test.go` — must be updated to assert predicates contain vendor=OpenShift
  - `TestCreateUpdatePlacement_CreatesNew` / `TestCreateUpdatePlacement_UpdatesExisting` — use explicit placement specs, so unaffected
  - `TestHandleComponentRightSizing_FeatureEnabled` — uses mockApplyChangesFunc, not placement creation, so unaffected
- **New regression test**: N/A — existing test is updated to cover the new expected behavior

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Code review of `GetDefaultRSPlacement()` shows empty Predicates; ticket confirms policy reaches non-OpenShift clusters |
| Approach correctness | HIGH | `ClusterPredicate` with `RequiredClusterSelector` is the standard OCM way to filter clusters; pattern confirmed in `helper_test.go` line 42-48 |
| Scope completeness | HIGH | Both namespace and virtualization components use `GetDefaultRSPlacement()` through `EnsureRSConfigMapExists` → default config generation, so fix is at the shared source |

### Investigation Strategy
**Signals detected**: environment (works on some cluster types, not others)
**Strategy used**: standard (grep/find/code-trace)
**Key findings from strategy**:
- `GetDefaultRSPlacement()` returns `Predicates: []clusterv1beta1.ClusterPredicate{}` (empty) at `placement.go:23`
- Both `rs-namespace` and `rs-virtualization` use `GetDefaultRSPlacement()` via their default config functions
- `TestGetDefaultRSPlacement` at `placement_test.go:22` asserts `Predicates` is empty — confirming this is the intended default, not a transient bug
