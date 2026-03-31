// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"context"
	"errors"
	"fmt"

	mcov1beta2 "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/api/v1beta2"
	rsnamespace "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/analytics/rightsizing/rs-namespace"
	rsutility "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility"
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

// CleanupPolicyResourcesForDelegation cleans up Policy-based right-sizing resources when delegating to MCOA.
// ConfigMaps are PRESERVED because MCOA owns them in the same namespace (open-cluster-management-observability).
// Only Policy, Placement, and PlacementBinding resources are cleaned up.
func CleanupPolicyResourcesForDelegation(
	ctx context.Context,
	c client.Client,
	mco *mcov1beta2.MultiClusterObservability,
) {
	log.Info("rs - cleaning up Policy resources for MCOA delegation (preserving ConfigMaps for MCOA)")

	// Use bindingUpdated=true to preserve ConfigMaps - MCOA owns them
	// Only clean up Policy, Placement, PlacementBinding resources
	if err := rsnamespace.CleanupRSNamespaceResources(ctx, c, rsnamespace.ComponentState.Namespace, true); err != nil {
		log.Error(err, "rs - failed to cleanup namespace Policy resources for delegation")
	}
	if err := rsvirtualization.CleanupRSVirtualizationResources(ctx, c, rsvirtualization.ComponentState.Namespace, true); err != nil {
		log.Error(err, "rs - failed to cleanup virtualization Policy resources for delegation")
	}

	log.Info("rs - Policy cleanup for MCOA delegation completed")
}

// getNamespaceBinding reads the NamespaceBinding from the MCO spec for a component type.
// Falls back to DefaultNamespace if not set. Uses MCO spec (not in-memory state) for robustness
// across operator restarts.
func getNamespaceBinding(mco *mcov1beta2.MultiClusterObservability, componentType rsutility.ComponentType) string {
	_, binding, err := rsutility.GetComponentConfig(mco, componentType)
	if err != nil || binding == "" {
		return rsutility.DefaultNamespace
	}
	return binding
}

// CleanupRightSizingResources cleans up ALL right-sizing resources (both namespace and virtualization).
// Called from the MCO finalizer during MCO CR deletion. Deletes everything including ConfigMaps.
// Reads namespace from MCO spec (not in-memory ComponentState) for robustness across operator restarts.
func CleanupRightSizingResources(
	ctx context.Context,
	c client.Client,
	mco *mcov1beta2.MultiClusterObservability,
) error {
	log.Info("rs - cleaning up all right-sizing resources (MCO deletion)")

	nsNamespace := getNamespaceBinding(mco, rsutility.ComponentTypeNamespace)
	virtNamespace := getNamespaceBinding(mco, rsutility.ComponentTypeVirtualization)

	// Use bindingUpdated=false to delete everything including ConfigMaps
	nsErr := rsnamespace.CleanupRSNamespaceResources(ctx, c, nsNamespace, false)
	virtErr := rsvirtualization.CleanupRSVirtualizationResources(ctx, c, virtNamespace, false)

	if err := errors.Join(nsErr, virtErr); err != nil {
		return fmt.Errorf("rs - failed to cleanup right-sizing resources: %w", err)
	}

	log.Info("rs - all right-sizing resources cleaned up")
	return nil
}
