// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rsnamespace

import (
	"context"

	mcov1beta2 "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/api/v1beta2"
	rsutility "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/multiclusterobservability/analytics/rs-utility"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// Namespace-specific resource names
	PlacementBindingName     = "rs-policyset-binding"
	PlacementName            = "rs-placement"
	PrometheusRulePolicyName = "rs-prom-rules-policy"
	PrometheusRuleName       = "acm-rs-namespace-prometheus-rules"
	ConfigMapName            = "rs-namespace-config"
)

// NamespaceManager encapsulates the component state and client
type NamespaceManager struct {
	Client client.Client
	State  *rsutility.ComponentState
}

// NewNamespaceManager creates a new NamespaceManager instance
func NewNamespaceManager(c client.Client) *NamespaceManager {
	return &NamespaceManager{
		Client: c,
		State: &rsutility.ComponentState{
			Namespace: rsutility.DefaultNamespace,
			Enabled:   false,
		},
	}
}

var (
	log = logf.Log.WithName("rs-namespace")
)

// buildComponentConfig creates the component configuration for this NamespaceManager
func (nm *NamespaceManager) buildComponentConfig(includeApplyChangesFunc bool) rsutility.ComponentConfig {
	config := rsutility.ComponentConfig{
		ComponentType:            rsutility.ComponentTypeNamespace,
		ConfigMapName:            ConfigMapName,
		PlacementName:            PlacementName,
		PlacementBindingName:     PlacementBindingName,
		PrometheusRulePolicyName: PrometheusRulePolicyName,
		DefaultNamespace:         rsutility.DefaultNamespace,
		GetDefaultConfigFunc:     GetDefaultRSNamespaceConfig,
	}

	if includeApplyChangesFunc {
		config.ApplyChangesFunc = func(ctx context.Context, c client.Client, configData rsutility.RSNamespaceConfigMapData) error {
			// Use this manager instance to apply changes
			return nm.ApplyRSNamespaceConfigMapChanges(ctx, configData)
		}
	}

	return config
}

// HandleRightSizing handles the namespace right-sizing functionality
func (nm *NamespaceManager) HandleRightSizing(ctx context.Context, mco *mcov1beta2.MultiClusterObservability) error {
	log.V(1).Info("rs - handling namespace right-sizing")

	// Use the common config builder with ApplyChangesFunc
	config := nm.buildComponentConfig(true)

	// Use generic component handler
	err := rsutility.HandleComponentRightSizing(ctx, nm.Client, mco, config, nm.State)
	return err
}

// GetRightSizingNamespaceConfig gets the namespace right-sizing configuration
func GetRightSizingNamespaceConfig(mco *mcov1beta2.MultiClusterObservability) (bool, string) {
	enabled, binding, err := rsutility.GetComponentConfig(mco, rsutility.ComponentTypeNamespace)
	if err != nil {
		log.Error(err, "rs - failed to get namespace right-sizing config")
		return false, ""
	}
	return enabled, binding
}

// CleanupRSNamespaceResources cleans up the resources created for namespace right-sizing
func (nm *NamespaceManager) CleanupRSNamespaceResources(ctx context.Context, namespace string, bindingUpdated bool) {
	log.V(1).Info("rs - cleaning up namespace resources if exist")

	// Use the common config builder without ApplyChangesFunc for cleanup
	config := nm.buildComponentConfig(false)

	rsutility.CleanupComponentResources(ctx, nm.Client, config, namespace, bindingUpdated)
}
