// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rsvirtualization

import (
	"context"

	rsutility "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/multiclusterobservability/analytics/rs-utility"
)

// CreateVirtualizationPlacementBinding creates the PlacementBinding resource for virtualization
func (vm *VirtualizationManager) CreateVirtualizationPlacementBinding(ctx context.Context) error {
	return rsutility.CreateRSPlacementBinding(ctx, vm.Client, PlacementBindingName, vm.State.Namespace, PlacementName, PrometheusRulePolicyName)
}
