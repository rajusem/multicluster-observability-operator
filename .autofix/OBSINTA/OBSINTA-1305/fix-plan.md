## Fix Plan for OBSINTA-1305

### Version
Plan v1 | Iteration 0 (initial draft)

### Root Cause
The `test-autofix-rs-placement-v10` branch does not contain the fix (commit `b3fa6f20`) that restricts right-sizing policies to OpenShift clusters only. This fix adds a `vendor=OpenShift` label selector to the Placement's cluster predicate, ensuring the `rs-prom-rules-policy` is only deployed to OpenShift clusters.

### Approach
Cherry-pick or apply the fix from commit `b3fa6f20` which:
1. Modifies `GetDefaultRSPlacement()` in `rs-utility/placement.go` to include a cluster predicate filtering for `vendor=OpenShift`
2. Updates the test in `rs-utility/placement_test.go` to verify this behavior

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Cherry-pick commit b3fa6f20 | Minimal changes, proven fix | Requires git operations | Not chosen - implementation decides |
| 2 | Rewrite with custom logic | Full control | More code, potential for bugs | Unnecessary - existing fix works |
| 3 | Add separate policy for OpenShift | Clear separation | Duplicate policy management | Overkill for this simple fix |

### Files to Change
| File | Change | Reason |
|------|--------|--------|
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` | Add `vendor=OpenShift` cluster predicate to `GetDefaultRSPlacement()` | Filter policies to OpenShift clusters only |
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement_test.go` | Update test to verify vendor=OpenShift selector | Ensure fix is tested |

### Dependencies & Side Effects
- [ ] Public API change? No
- [ ] Config / env var change? No
- [ ] Database migration? No
- [ ] Downstream consumer impact? No - only restricts where policies are deployed
- [ ] Error handling / logging change? No
- [ ] Performance characteristics change? No

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Cherry-pick conflicts | Low | Medium | Manual resolution if needed |
| Test failure | Low | Low | Unit tests verify fix |

### Test Strategy
- Existing tests to verify: Run `make unit-tests` for the rightsizing packages
- New regression test: Update existing test to verify vendor=OpenShift filter

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Commit b3fa6f20 exists in main but not in ticket branch |
| Approach correctness | HIGH | Same fix was already merged and tested |
| Scope completeness | HIGH | Only 2 files need change |

### Investigation Strategy
**Signals detected**: environment (works in one environment but not another - AKS vs OpenShift)
**Strategy used**: standard investigation
**Key findings from strategy**:
- The issue is that the right-sizing policies are being deployed to all clusters including AKS
- Commit b3fa6f20 adds the vendor=OpenShift filter to the Placement's `GetDefaultRSPlacement()` function
- The fix was already merged to main but is missing from the ticket's branch `test-autofix-rs-placement-v10`

### Commit Reference
The fix to cherry-pick: `b3fa6f20` - "Restrict right-sizing policies to OpenShift clusters only (#2447)"