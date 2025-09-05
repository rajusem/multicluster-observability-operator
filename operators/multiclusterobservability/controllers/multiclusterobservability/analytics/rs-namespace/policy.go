// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rsnamespace

import (
	"context"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rsutility "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/multiclusterobservability/analytics/rs-utility"
)

// CreateOrUpdatePrometheusRulePolicy creates or updates the PrometheusRule policy
func (nm *NamespaceManager) CreateOrUpdatePrometheusRulePolicy(
	ctx context.Context,
	prometheusRule monitoringv1.PrometheusRule,
) error {
	return rsutility.CreateOrUpdateRSPrometheusRulePolicy(ctx, nm.Client, PrometheusRulePolicyName, nm.State.Namespace, prometheusRule)
}
