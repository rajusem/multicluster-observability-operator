## Fix Plan for OBSINTA-1365

### Version
Plan v1 | Audit skipped (all HIGH confidence, 2 files, <20 lines)

### Root Cause
`GetDefaultRSPlacement()` in `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` (line 23) initializes `Predicates` as an empty slice (`[]clusterv1beta1.ClusterPredicate{}`). This causes the Placement resource—which controls where `rs-prom-rules-policy` and `rs-virt-prom-rules-policy` are deployed—to match ALL managed clusters. Non-OpenShift clusters (e.g., AKS) lack the `openshift-monitoring` namespace that the PrometheusRule ConfigurationPolicy targets, causing failures on those clusters.

The fix commit `b3fa6f20` ("Restrict right-sizing policies to OpenShift clusters only") exists on `main` but is **not** present in the `issue-fix-OBSINTA-1329` base branch used for this evaluation.

### Approach
Add a `vendor=OpenShift` label selector (`MatchExpressions`) to the `Predicates` slice in `GetDefaultRSPlacement()`. This ensures the placement only selects clusters with `vendor=OpenShift` label—effectively restricting policy deployment to OpenShift clusters only. Also update `TestGetDefaultRSPlacement` in `placement_test.go` to assert the predicate is present and correct.

This is the same approach as commit `b3fa6f20` merged to `main`, re-applied to the `issue-fix-OBSINTA-1329` branch.

### Planned Files

- `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` — Replace empty `Predicates: []clusterv1beta1.ClusterPredicate{}` with a single `ClusterPredicate` containing `RequiredClusterSelector.LabelSelector.MatchExpressions` filtering `vendor In [OpenShift]`

  **Exact change** (mirrors commit `b3fa6f20`):
  ```go
  // Before:
  Predicates: []clusterv1beta1.ClusterPredicate{},

  // After:
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
  Also update the function comment to explain WHY (openshift-monitoring only exists on OCP).

- `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go` — Update `TestGetDefaultRSPlacement`:
  - Change `assert.Empty(t, placement.Spec.Predicates)` → `assert.Len(t, placement.Spec.Predicates, 1)`
  - Add assertions: predicate key = `"vendor"`, operator = `metav1.LabelSelectorOpIn`, values = `[]string{"OpenShift"}`

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Add `vendor=OpenShift` predicate to `GetDefaultRSPlacement()` (chosen) | Minimal change, proven fix | None | Selected — simplest correct fix |
| 2 | Add cluster filter in Policy `NamespaceSelector` | Restricts namespace scope | Does not prevent policy propagation to non-OCP clusters | Wrong layer |
| 3 | Add runtime check before creating policy | Could skip creation | More complex, changes control flow | Unnecessary |

### Dependencies & Side Effects
- Public API change? No — `GetDefaultRSPlacement()` is internal
- Config / env var change? No
- Database migration? No
- Downstream consumer impact? Yes — existing non-OpenShift clusters will stop receiving the policy (desired behavior per the bug fix)
- Error handling / logging change? No
- Performance characteristics change? No

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Existing OCP clusters without vendor label | Low | Medium | ACM auto-labels managed clusters with `vendor` on import |
| ConfigMap already seeded without predicate | Medium | Low | Only affects default initialization; operator restart or manual CM update fixes it |

### Test Strategy
- Existing tests: `make unit-tests` in `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/`
- New regression test: Updated `TestGetDefaultRSPlacement` verifies `vendor=OpenShift` predicate is present

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | `placement.go:23` shows empty `Predicates`. Commit `b3fa6f20` confirms fix location. |
| Approach correctness | HIGH | Same fix already merged and proven on `main` |
| Scope completeness | HIGH | Only 2 files. `GetDefaultRSPlacement()` called only from `rs-namespace/configmap.go` and `rs-virtualization/configmap.go` |

### Audit Trail
- Audit: **skipped** — simple fix, all confidence HIGH, ≤2 files, <20 lines
- Investigation strategy: environment signal + git history (regression: fix on main, missing from branch)
