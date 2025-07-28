// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package analytics

import (
	"fmt"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// — constants for metadata
const (
	vmPrometheusRuleName  = "acm-vm-right-sizing-rules"
	vmMonitoringNamespace = "openshift-monitoring"
)

// assume you have a config struct similar to RSNamespaceConfigMapData but with VM criteria:
type RSVMNamespaceConfigMapData struct {
	PrometheusRuleConfig RSPrometheusRuleConfig
}

// entry‑point: build the VM PrometheusRule
func generateVMPrometheusRule(configData RSVMNamespaceConfigMapData) (monitoringv1.PrometheusRule, error) {
	// 1. namespace filter excludes openshift.* and xyz.*
	nsFilter, err := buildVMNamespaceFilter(configData.PrometheusRuleConfig)
	if err != nil {
		return monitoringv1.PrometheusRule{}, err
	}

	// 2. label join on label_env & kubernetes_io_metadata_name
	labelJoin, err := buildVMLabelJoin(configData.PrometheusRuleConfig.LabelFilterCriteria)
	if err != nil {
		return monitoringv1.PrometheusRule{}, err
	}

	// durations: 5m / 15m (for your “1d” group)
	d5 := monitoringv1.Duration("5m")
	d1 := monitoringv1.Duration("15m")

	// helper factories
	vmRule := func(record, expr string) monitoringv1.Rule {
		fullExpr := expr
		if labelJoin != "" {
			fullExpr = fmt.Sprintf("%s * on (namespace) group_left() (%s)", expr, labelJoin)
		}
		return monitoringv1.Rule{
			Record: record,
			Expr:   intstr.FromString(fullExpr),
		}
	}
	vmRuleWithLabels := func(record, expr string) monitoringv1.Rule {
		return monitoringv1.Rule{
			Record: record,
			Expr:   intstr.FromString(expr),
			Labels: map[string]string{
				"aggregation": "1d",
				"profile":     "Max OverAll",
			},
		}
	}

	return monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmPrometheusRuleName,
			Namespace: vmMonitoringNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "PrometheusRule",
			APIVersion: "monitoring.coreos.com/v1",
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name:     "acm-vm-right-sizing-namespace-5m.rule",
					Interval: &d5,
					Rules:    buildVMNamespaceRules5m(nsFilter, vmRule),
				},
				{
					Name:     "acm-vm-right-sizing-namespace-1d.rules",
					Interval: &d1,
					Rules:    buildVMNamespaceRules1d(configData, vmRuleWithLabels),
				},
				{
					Name:     "acm-vm-right-sizing-cluster-5m.rule",
					Interval: &d5,
					Rules:    buildVMClusterRules5m(nsFilter, vmRule),
				},
				{
					Name:     "acm-vm-right-sizing-cluster-1d.rules",
					Interval: &d1,
					Rules:    buildVMClusterRules1d(configData, vmRuleWithLabels),
				},
			},
		},
	}, nil
}

// build a namespace filter matching your YAML: exclude openshift.* or xyz.*
func buildVMNamespaceFilter(nsConfig RSPrometheusRuleConfig) (string, error) {
	// force exclusion only
	excl := nsConfig.NamespaceFilterCriteria.ExclusionCriteria
	if len(excl) == 0 {
		return "", fmt.Errorf("must specify exclusionCriteria for VM namespaces")
	}
	return fmt.Sprintf(`namespace!~"%s"`, strings.Join(excl, "|")), nil
}

// build the two‑part label join on label_env and kubernetes_io_metadata_name
func buildVMLabelJoin(filters []RSLabelFilter) (string, error) {
	var envCriteria, nameCrit []string
	for _, f := range filters {
		switch f.LabelName {
		case "label_env":
			if len(f.InclusionCriteria) > 0 {
				envCriteria = f.InclusionCriteria
			}
		case "label_kubernetes_io_metadata_name":
			if len(f.ExclusionCriteria) > 0 {
				nameCrit = f.ExclusionCriteria
			}
		}
	}
	if len(envCriteria) == 0 {
		return "", nil
	}
	// build the inner selector, plus allow empty
	selector := fmt.Sprintf(
		`kube_namespace_labels{label_env=~"%s", label_kubernetes_io_metadata_name!~"%s"}`,
		strings.Join(envCriteria, "|"),
		strings.Join(nameCrit, "|"),
	)
	return fmt.Sprintf(`%s or kube_namespace_labels{label_env=""}`, selector), nil
}

// namespace 5m rules for VM
func buildVMNamespaceRules5m(
	nsFilter string,
	rule func(string, string) monitoringv1.Rule,
) []monitoringv1.Rule {
	return []monitoringv1.Rule{
		rule(
			"acm_rs_vm:namespace:cpu_request:5m",
			fmt.Sprintf(
				`max_over_time(sum((
				  kubevirt_vm_resource_requests{%s, unit="cores", resource="cpu"} *
				  on(name,namespace,resource)
				  kubevirt_vm_resource_requests{%s, unit="sockets", resource="cpu"} *
				  on(name,namespace,resource)
				  kubevirt_vm_resource_requests{%s, unit="threads", resource="cpu"}
				) by(name,namespace)[5m:])`,
				nsFilter, nsFilter, nsFilter,
			),
		),
		rule(
			"acm_rs_vm:namespace:memory_request:5m",
			fmt.Sprintf(
				`max_over_time(sum(
				  kubevirt_vm_resource_requests{%s, resource="memory"}
				) by(name,namespace)[5m:])`,
				nsFilter,
			),
		),
		rule(
			"acm_rs_vm:namespace:cpu_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum(
				  rate(kubevirt_vmi_cpu_usage_seconds_total{%s}[5m:])
				) by(name,namespace)[5m:])`,
				nsFilter,
			),
		),
		rule(
			"acm_rs_vm:namespace:memory_usage:5m",
			fmt.Sprintf(
				`max_over_time(sum(
				  kubevirt_vmi_memory_available_bytes{%s} -
				  kubevirt_vmi_memory_usable_bytes{%s}
				) by(name,namespace)[5m:])`,
				nsFilter, nsFilter,
			),
		),
	}
}

// namespace 1d rules with recommendations
func buildVMNamespaceRules1d(
	configData RSVMNamespaceConfigMapData,
	ruleWithLabels func(string, string) monitoringv1.Rule,
) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	return []monitoringv1.Rule{
		ruleWithLabels("acm_rs_vm:namespace:cpu_request", `max_over_time(acm_rs_vm:namespace:cpu_request:5m[1d])`),
		ruleWithLabels("acm_rs_vm:namespace:cpu_usage", `max_over_time(acm_rs_vm:namespace:cpu_usage:5m[1d])`),
		ruleWithLabels("acm_rs_vm:namespace:memory_request", `max_over_time(acm_rs_vm:namespace:memory_request:5m[1d])`),
		ruleWithLabels("acm_rs_vm:namespace:memory_usage", `max_over_time(acm_rs_vm:namespace:memory_usage:5m[1d])`),
		ruleWithLabels(
			"acm_rs_vm:namespace:cpu_recommendation",
			fmt.Sprintf(`max_over_time(acm_rs_vm:namespace:cpu_usage{profile="Max OverAll"}[1d])*(1+(%d/100))`, rp),
		),
		ruleWithLabels(
			"acm_rs_vm:namespace:memory_recommendation",
			fmt.Sprintf(`max_over_time(acm_rs_vm:namespace:memory_usage{profile="Max OverAll"}[1d])*(1+(%d/100))`, rp),
		),
	}
}

// cluster‑scoped 5m
func buildVMClusterRules5m(
	nsFilter string,
	rule func(string, string) monitoringv1.Rule,
) []monitoringv1.Rule {
	// same as namespace but group by cluster
	rules := buildVMNamespaceRules5m(nsFilter, rule)
	for i := range rules {
		rules[i].Record = strings.Replace(rules[i].Record, ":namespace:", ":cluster:", 1)
	}
	return rules
}

// cluster‑scoped 1d
func buildVMClusterRules1d(
	configData RSVMNamespaceConfigMapData,
	ruleWithLabels func(string, string) monitoringv1.Rule,
) []monitoringv1.Rule {
	rules := buildVMNamespaceRules1d(configData, ruleWithLabels)
	for i := range rules {
		rules[i].Record = strings.Replace(rules[i].Record, ":namespace:", ":cluster:", 1)
	}
	return rules
}
