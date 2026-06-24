# Fix Plan for OBSINTA-1320

### Version
Plan v1 | Iteration 0 (initial draft) — APPROVED, audit skipped

### Root Cause
The `GetDefaultRSPlacement()` function in `rs-utility/placement.go` creates a Placement resource with an **empty `Predicates` slice** (`Predicates: []clusterv1beta1.ClusterPredicate{}`). Because the OCM Cluster Federation Placement controller uses predicates to determine which clusters a policy should be bound to, an empty predicate list means the placement matches **all clusters including AKS**. The `rs-prom-rules-policy` is therefore applied to every managed cluster rather than only OpenShift clusters.

In `placement.go` line 23:
```go
Predicates: []clusterv1beta1.ClusterPredicate{},  // matches ALL clusters
```

The right-sizing Prometheus rules should only be applied when the cluster's `vendor` label equals `"OpenShift"`, as OpenShift-specific monitoring namespaces and resources are the target.

### Approach
Add a `ClusterPredicate` with a `LabelSelector` that filters `vendor=OpenShift` to the Placement's `Predicates` list. The predicate uses standard Kubernetes label selector matching, which is clean and idiomatic — the OCM placement controller will automatically evaluate the selector against each ManagedCluster's labels and only match clusters where `vendor=OpenShift`.

This aligns with the existing pattern in the codebase where `areManagedClusterLabelsReady()` (placementrule_controller.go:799) checks `vendor == "OpenShift"` to gate OpenShift-specific behavior.

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Use `ClusterProviderPredicate` with `Hypershift` predicate | Would explicitly filter | Too narrow — only catches Hypershift, not generic non-OpenShift (AKS) | Doesn't actually solve the problem |
| 2 | Filter in the policy's `ConfigurationPolicy.NamespaceSelector` | Works for namespace targeting | NamespaceSelector controls target namespaces, not target clusters | Wrong axis of filtering |
| 3 | Add predicate to both Predicates and ClusterProviderPredicates | Broad coverage | Unnecessary complexity for this problem | Single label selector is sufficient |

### Files to Change
| File | Change | Reason |
|------|--------|--------|
| `operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility/placement.go` | Add `ClusterPredicate` with `LabelSelector` for `vendor=OpenShift` | Restricts rs-prom-rules-policy to OpenShift clusters only |

### Dependencies & Side Effects
- [ ] Public API change? No — placement config is internal
- [ ] Config / env var change? No — in-code fix
- [ ] Database migration? N/A
- [ ] Downstream consumer impact? Right-sizing rules now only apply to OpenShift clusters (intended behavior)
- [ ] Error handling / logging change? No
- [ ] Performance characteristics change? Slight improvement — fewer clusters evaluated

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| No clusters match (vendor label absent) | LOW | Policy not deployed anywhere | Vendor label is standard on all MCO-managed clusters |
| Breaking existing AKS monitoring workflows | MEDIUM | AKS right-sizing data unavailable | AKS was already receiving incorrect rules; removing them is the correct behavior |

### Test Strategy
- Existing tests in `helper_test.go` verify YAML serialization — the placement structure must still serialize correctly to YAML
- The unit test `TestFormatYAML_WithPlacement` already covers predicate serialization
- No new tests needed for this change (it's a configuration value change, not new logic)

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | Empty predicates → matches all clusters is OCM placement controller behavior |
| Approach correctness | HIGH | LabelSelector-based filtering is the standard OCM pattern for cluster selection |
| Scope completeness | HIGH | Single-file change, no downstream dependencies |

### Investigation Strategy
**Signals detected**: environment (works in OpenShift, incorrectly applies to AKS)
**Strategy used**: Standard investigation — grep + code path tracing
**Key findings from strategy**:
- `GetDefaultRSPlacement()` creates placement with empty predicates
- `CreateUpdateRSPlacement()` directly passes this spec to the OCM Placement API
- Empty predicates → Cluster Federation Placement controller selects ALL clusters
