// Package-level constants, variables and helpers shared by rsnamespace package.
package rsnamespace

import (
	"gopkg.in/yaml.v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Constants duplicated from the parent analytics package. Keeping the same
// literal values avoids an import cycle while preserving behaviour.
const (
	RSPolicySetName                  = "rs-policyset"
	RSPlacementName                  = "rs-placement"
	RSPlacementBindingName           = "rs-policyset-binding"
	RSPrometheusRulePolicyName       = "rs-prom-rules-policy"
	RSPrometheusRulePolicyConfigName = "rs-prometheus-rules-policy-config"
	RSPrometheusRuleName             = "acm-rs-namespace-prometheus-rules"
	RSConfigMapName                  = "rs-namespace-config"
	RSMonitoringNamespace            = "openshift-monitoring"

	// DefaultNamespace is kept for completeness although the value can be
	// overridden at runtime via SetNamespace.
	DefaultNamespace = "open-cluster-management-global-set"

	RSDefaultRecommendationPercentage = 110
)

var (
	// log is a package-scoped logger dedicated to rsnamespace code paths.
	log = logf.Log.WithName("analytics-rsnamespace")

	currentNamespace = DefaultNamespace

	// Lower-case aliases preserve the original identifiers used in the
	// codebase, so we don't have to touch every reference after the move.
	rsPolicySetName                  = RSPolicySetName
	rsPlacementName                  = RSPlacementName
	rsPlacementBindingName           = RSPlacementBindingName
	rsPrometheusRulePolicyName       = RSPrometheusRulePolicyName
	rsPrometheusRulePolicyConfigName = RSPrometheusRulePolicyConfigName
	rsPrometheusRuleName             = RSPrometheusRuleName
	rsConfigMapName                  = RSConfigMapName
	rsMonitoringNamespace            = RSMonitoringNamespace

	rsDefaultRecommendationPercentage = RSDefaultRecommendationPercentage
)

// SetNamespace updates the namespace used when creating Placement, Policy, etc.
// This is called by the parent analytics package whenever the binding changes.
func SetNamespace(ns string) {
	if ns != "" {
		currentNamespace = ns
	}
}

// GetNamespace returns the namespace currently in use for right-sizing resources.
func GetNamespace() string {
	return currentNamespace
}

// FormatYAML converts a Go data structure to a YAML-formatted string. A local
// copy is kept here to avoid importing the parent analytics package (which
// would create an import cycle).
func FormatYAML[T any](data T) string {
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		log.Error(err, "rs - error marshaling data to yaml")
		return ""
	}
	return string(yamlData)
}
