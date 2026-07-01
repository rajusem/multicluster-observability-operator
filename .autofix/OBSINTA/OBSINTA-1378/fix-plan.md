## Fix Plan for OBSINTA-1378

### Version
Plan v1 | Audit skipped (simple fix — all confidence HIGH, 1 file, ~2 lines)

### Root Cause
The "ACM Right-Sizing OpenShift Virtualization" main Grafana dashboard (`dash-acm-right-sizing-virtualization.yaml`) has the `timepicker` property set to `{"hidden": true}` (line 2807-2808 of the YAML file). In Grafana, when `timepicker.hidden` is `true`, it hides the entire time picker toolbar area — which includes the refresh button. All other right-sizing dashboards (namespace, overestimation, underestimation) have `"timepicker": {}` (an empty object), which keeps the timepicker and refresh button visible.

### Approach
Change `"timepicker": {"hidden": true}` to `"timepicker": {}` in the virtualization main dashboard JSON definition. This is consistent with every other right-sizing dashboard in the same directory and will restore the refresh button to the dashboard toolbar.

### Alternatives Considered
| # | Approach | Pros | Cons | Why Not |
|---|----------|------|------|---------|
| 1 | Set `"timepicker": {"hidden": false}` | Explicitly shows timepicker | Non-standard — other dashboards use empty `{}` | Less consistent with existing dashboards |
| 2 | Add a custom refresh panel widget | Full control over refresh UI | Unnecessary complexity, non-standard | Overkill — built-in refresh button is the standard approach |

### Files to Change
| File | Change | Reason |
|------|--------|--------|
| `operators/multiclusterobservability/manifests/base/grafana/analytics/dash-acm-right-sizing-virtualization.yaml` | Change `"timepicker": {"hidden": true}` (lines 2807-2808) to `"timepicker": {}` | Removes the hidden flag that suppresses the refresh button and time picker in the dashboard toolbar |

### Dependencies & Side Effects
- [ ] Public API change? — No
- [ ] Config / env var change? — No
- [ ] Database migration? — No
- [ ] Downstream consumer impact? — No (only affects Grafana dashboard UI)
- [ ] Error handling / logging change? — No
- [ ] Performance characteristics change? — No

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Time picker visibility changes dashboard UX | LOW | LOW | This matches all other right-sizing dashboards; refresh and time range controls are expected |

### Test Strategy
- Existing tests to verify: Dashboard loading tests (if any exist in `tests/pkg/tests/observability_grafana_test.go`)
- New regression test: None needed — this is a configuration-only change in a Grafana JSON dashboard
- Manual verification: Load the dashboard in Grafana and confirm the refresh button appears in the toolbar

### Confidence
| Dimension | Score | Proof |
|-----------|-------|-------|
| Root cause certainty | HIGH | `timepicker.hidden=true` is the only property difference between this dashboard (no refresh button) and all 3 other right-sizing dashboards (with refresh button). Grafana docs confirm `timepicker.hidden` controls toolbar visibility. |
| Approach correctness | HIGH | Setting `"timepicker": {}` matches the exact pattern used by `dash-acm-right-sizing-namespace.yaml` (line 2028), `dash-acm-right-sizing-virtualization-overestimation.yaml` (line 1175), and `dash-acm-right-sizing-virtualization-underestimation.yaml` (line 1175). |
| Scope completeness | HIGH | Only 1 file needs to change. The change is a 2-line edit (replacing 2 lines with 1). No other files reference or depend on the timepicker hidden setting. |

### Investigation Strategy
**Signals detected**: default
**Strategy used**: Standard investigation (grep, file comparison)
**Key findings from strategy**:
  - Compared the virtualization main dashboard with the namespace dashboard and the two sub-dashboards (overestimation, underestimation)
  - The only structural difference in the dashboard metadata is `"timepicker": {"hidden": true}` vs `"timepicker": {}`
  - All other dashboards in the same folder have visible timepickers and refresh buttons
