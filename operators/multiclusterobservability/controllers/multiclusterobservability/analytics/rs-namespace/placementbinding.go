// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rsnamespace

import (
	"context"

	rsutility "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/multiclusterobservability/analytics/rs-utility"
)

// CreatePlacementBinding creates the PlacementBinding resource
func (nm *NamespaceManager) CreatePlacementBinding(ctx context.Context) error {
	return rsutility.CreateRSPlacementBinding(ctx, nm.Client, PlacementBindingName, nm.State.Namespace, PlacementName, PrometheusRulePolicyName)
}
