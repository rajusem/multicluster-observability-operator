// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package analytics

import (
	"context"
	"fmt"

	mcov1beta2 "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/api/v1beta2"
	rsnamespace "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/multiclusterobservability/analytics/rs-namespace"
	rsvirtualization "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/multiclusterobservability/analytics/rs-virtualization"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	log = logf.Log.WithName("analytics")
)

var (
	// Global manager instances - created once and reused
	namespaceManager      *rsnamespace.NamespaceManager
	virtualizationManager *rsvirtualization.VirtualizationManager
)

func CreateRightSizingComponent(
	ctx context.Context,
	c client.Client,
	mco *mcov1beta2.MultiClusterObservability,
) error {
	log.V(1).Info("rs - inside create rs component")

	// Initialize managers if not already created
	if namespaceManager == nil {
		namespaceManager = rsnamespace.NewNamespaceManager(c)
	}
	if virtualizationManager == nil {
		virtualizationManager = rsvirtualization.NewVirtualizationManager(c)
	}

	// Handle namespace right-sizing using the singleton manager
	if err := namespaceManager.HandleRightSizing(ctx, mco); err != nil {
		return fmt.Errorf("failed to handle namespace right-sizing: %w", err)
	}

	// Handle virtualization right-sizing using the singleton manager
	if err := virtualizationManager.HandleRightSizing(ctx, mco); err != nil {
		return fmt.Errorf("failed to handle virtualization right-sizing: %w", err)
	}

	log.Info("rs - create component task completed")
	return nil
}

// GetNamespaceRSConfigMapPredicateFunc returns predicate for namespace right-sizing ConfigMap
func GetNamespaceRSConfigMapPredicateFunc(ctx context.Context, c client.Client) predicate.Funcs {
	// Initialize manager if not already created
	if namespaceManager == nil {
		namespaceManager = rsnamespace.NewNamespaceManager(c)
	}
	return rsnamespace.GetNamespaceRSConfigMapPredicateFunc(ctx, namespaceManager)
}

// GetVirtualizationRSConfigMapPredicateFunc returns predicate for virtualization right-sizing ConfigMap
func GetVirtualizationRSConfigMapPredicateFunc(ctx context.Context, c client.Client) predicate.Funcs {
	// Initialize manager if not already created
	if virtualizationManager == nil {
		virtualizationManager = rsvirtualization.NewVirtualizationManager(c)
	}
	return rsvirtualization.GetVirtualizationRSConfigMapPredicateFunc(ctx, virtualizationManager)
}
