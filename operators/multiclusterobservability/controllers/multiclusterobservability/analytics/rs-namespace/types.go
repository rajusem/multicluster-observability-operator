package rsnamespace

import (
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
)

// RSLabelFilter mirrors the definition in the parent analytics package.
// It is duplicated here to avoid an import cycle.
type RSLabelFilter struct {
	LabelName         string   `yaml:"labelName"`
	InclusionCriteria []string `yaml:"inclusionCriteria,omitempty"`
	ExclusionCriteria []string `yaml:"exclusionCriteria,omitempty"`
}

// RSPrometheusRuleConfig is the YAML-driven configuration for generating the
// right-sizing PrometheusRule objects.
type RSPrometheusRuleConfig struct {
	NamespaceFilterCriteria struct {
		InclusionCriteria []string `yaml:"inclusionCriteria"`
		ExclusionCriteria []string `yaml:"exclusionCriteria"`
	} `yaml:"namespaceFilterCriteria"`
	LabelFilterCriteria      []RSLabelFilter `yaml:"labelFilterCriteria"`
	RecommendationPercentage int             `yaml:"recommendationPercentage"`
}

// RSNamespaceConfigMapData represents the data section of the rs-namespace
// ConfigMap.
type RSNamespaceConfigMapData struct {
	PrometheusRuleConfig   RSPrometheusRuleConfig   `yaml:"prometheusRuleConfig"`
	PlacementConfiguration clusterv1beta1.Placement `yaml:"placementConfiguration"`
}
