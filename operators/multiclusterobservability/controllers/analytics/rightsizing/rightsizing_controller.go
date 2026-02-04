// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"context"
	"fmt"

	mcov1beta2 "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/api/v1beta2"
	rsnamespace "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/analytics/rightsizing/rs-namespace"
	rsvirtualization "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/analytics/rightsizing/rs-virtualization"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var log = logf.Log.WithName("analytics")

func CreateRightSizingComponent(
	ctx context.Context,
	c client.Client,
	mco *mcov1beta2.MultiClusterObservability,
) error {
	log.V(1).Info("rs - inside create rs component")

	// Handle namespace right-sizing
	if err := rsnamespace.HandleRightSizing(ctx, c, mco); err != nil {
		return fmt.Errorf("failed to handle namespace right-sizing: %w", err)
	}

	// Handle virtualization right-sizing
	if err := rsvirtualization.HandleRightSizing(ctx, c, mco); err != nil {
		return fmt.Errorf("failed to handle virtualization right-sizing: %w", err)
	}

	log.Info("rs - create component task completed")
	return nil
}

// GetNamespaceRSConfigMapPredicateFunc returns predicate for namespace right-sizing ConfigMap
func GetNamespaceRSConfigMapPredicateFunc(ctx context.Context, c client.Client) predicate.Funcs {
	return rsnamespace.GetNamespaceRSConfigMapPredicateFunc(ctx, c)
}

// GetVirtualizationRSConfigMapPredicateFunc returns predicate for virtualization right-sizing ConfigMap
func GetVirtualizationRSConfigMapPredicateFunc(ctx context.Context, c client.Client) predicate.Funcs {
	return rsvirtualization.GetVirtualizationRSConfigMapPredicateFunc(ctx, c)
}

// CleanupPolicyResourcesForDelegation cleans up Policy resources when delegating to MCOA.
// ConfigMaps are preserved so MCOA can read the configuration.
// This function uses bindingUpdated=true to keep ConfigMaps while cleaning up Policy/Placement/PlacementBinding.
func CleanupPolicyResourcesForDelegation(
	ctx context.Context,
	c client.Client,
	mco *mcov1beta2.MultiClusterObservability,
) {
	log.Info("rs - cleaning up policy resources for MCOA delegation")

	// Use existing cleanup functions with bindingUpdated=true to preserve ConfigMaps
	// These use ComponentState.Namespace which tracks the current namespace binding
	rsnamespace.CleanupRSNamespaceResources(ctx, c, rsnamespace.ComponentState.Namespace, true)
	rsvirtualization.CleanupRSVirtualizationResources(ctx, c, rsvirtualization.ComponentState.Namespace, true)

	log.Info("rs - policy cleanup for MCOA delegation completed")
}
