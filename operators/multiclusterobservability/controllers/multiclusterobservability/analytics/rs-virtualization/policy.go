// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rsvirtualization

import (
	"context"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rsutility "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/multiclusterobservability/analytics/rs-utility"
)

// CreateOrUpdateVirtualizationPrometheusRulePolicy creates or updates the PrometheusRule policy for virtualization
func (vm *VirtualizationManager) CreateOrUpdateVirtualizationPrometheusRulePolicy(
	ctx context.Context,
	prometheusRule monitoringv1.PrometheusRule,
) error {
	return rsutility.CreateOrUpdateRSPrometheusRulePolicy(ctx, vm.Client, PrometheusRulePolicyName, vm.State.Namespace, prometheusRule)
}
