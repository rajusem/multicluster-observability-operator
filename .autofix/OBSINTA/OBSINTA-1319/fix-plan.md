## Fix Plan for OBSINTA-1319

### Version
Plan v1 | Iteration 0 (initial draft)

### Root Cause
The `test-autofix-e2e-final` branch diverges from an older commit (`877d167b`) that predates the fix from OBSINTA-1316 (commit `95df4604`). On this branch, `GetDefaultRSPlacement()` in `rs-utility/placement.go` returns a `Placement` with `Predicates: []clusterv1beta1.ClusterPredicate{}` — an empty predicates list. An empty predicate list selects ALL managed clusters, including non-OpenShift distributions like AKS and GKE.

The `rs-prom-rules-policy` targets the `openshift-monitoring` namespace (constant `MonitoringNamespace = "openshift-monitoring"` in `types.go`). Since non-OpenShift clusters lack this namespace, the ConfigurationPolicy enforcement fails, causing policy compliance issues on AKS and other non-OpenShift clusters.

### Approach
Replace the empty `Predicates` slice with a `ClusterPredicate` that uses `LabelSelectorOperator: In` with `Key: "vendor"` and `Values: ["OpenShift"]`. This filters the placement to only target OpenShift-managed clusters, preventing the policy from being enforced on AKS/GKE where `openshift-monitoring` does not exist.

This is the same fix applied in commit `95df4604` for OBSINTA-1316, adapted to the current branch state.

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Use `MatchLabels` (`vendor: OpenShift`) instead of `MatchExpressions` | Simpler syntax | Less flexible for future changes | The MatchExpressions form is already the standard used across the codebase and matches the existing pattern in main branch |
| 2 | Remove the policy entirely for non-OpenShift clusters | Fixes the symptom | Loses right-sizing metrics on OpenShift clusters; over-complicated | The root cause is the placement, not the policy itself. Filtering at placement level is the correct approach |

### Files to Change
| File | Change | Reason |
|------|--------|--------|
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` | Replace `Predicates: []clusterv1beta1.ClusterPredicate{}` with a ClusterPredicate containing `vendor=OpenShift` label selector | Root cause fix — restrict placement to OpenShift clusters only |
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go` | Add test assertions verifying the vendor=OpenShift predicate exists and has correct match expression | Validate the fix in tests |

### Dependencies & Side Effects
- [ ] Public API change? No — only changes internal placement filter
- [ ] Config / env var change? No — no config changes needed (the placement is created via the Go struct)
- [ ] Database migration? No
- [ ] Downstream consumer impact? No — this is a filter that narrows scope; existing OpenShift clusters unaffected
- [ ] Error handling / logging change? No
- [ ] Performance characteristics change? Negligible — predicate evaluation is O(1) per cluster

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Broken placement binding due to type mismatch | Low | Medium | The types used (`ClusterPredicate`, `ClusterSelector`, `LabelSelectorRequirement`) are the same existing types already in use |
| Test failure due to missing `MatchExpressions` type | Low | Low | Standard Kubernetes apimachinery types — well-tested |

### Test Strategy
- Existing test `TestGetDefaultRSPlacement` should be updated to verify the predicate exists and contains the correct `vendor=OpenShift` match expression
- Run `go test ./operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/... -run TestGetDefaultRSPlacement -v`

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Confirmed by comparing test branch placement.go (empty predicates) with main branch (has vendor=OpenShift predicate). The fix commit 95df4604 matches the same change pattern. |
| Approach correctness | HIGH | Vendor label is the standard way to identify OpenShift clusters in ACM/KubeVirt ecosystem. Same approach used in main branch. |
| Scope completeness | HIGH | Only 2 files affected: one source change, one test update. No cross-module dependencies. |

### Investigation Strategy
**Signals detected**: regression
**Strategy used**: Standard investigation (diff against main branch + code path tracing)
**Key findings from strategy**:
- The `test-autofix-e2e-final` branch is based on commit `877d167b`, which precedes the fix in `95df4604`
- The same root cause was already identified and fixed for OBSINTA-1316 on a different branch
- The fix is a single structural change: replacing empty Predicates slice with vendor=OpenShift ClusterPredicate
- The policy target namespace (`openshift-monitoring`) and the namespace selector are correct; only the placement predicate is missing
