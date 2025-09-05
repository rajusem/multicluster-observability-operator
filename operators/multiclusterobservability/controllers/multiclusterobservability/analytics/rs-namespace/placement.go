// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rsnamespace

import (
	"context"

	rsutility "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/multiclusterobservability/analytics/rs-utility"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
)

// CreateUpdatePlacement creates the Placement resource
func (nm *NamespaceManager) CreateUpdatePlacement(ctx context.Context, placementConfig clusterv1beta1.Placement) error {
	return rsutility.CreateUpdateRSPlacement(ctx, nm.Client, PlacementName, nm.State.Namespace, placementConfig)
}
