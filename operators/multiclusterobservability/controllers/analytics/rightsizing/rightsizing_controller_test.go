// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package rightsizing

import (
	"context"
	"testing"

	mcov1beta2 "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/api/v1beta2"
	rsnamespace "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/analytics/rightsizing/rs-namespace"
	rsutility "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/analytics/rightsizing/rs-utility"
	rsvirtualization "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/analytics/rightsizing/rs-virtualization"
	mcoconfig "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func setupTestScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, mcov1beta2.AddToScheme(scheme))
	require.NoError(t, policyv1.AddToScheme(scheme))
	require.NoError(t, clusterv1beta1.AddToScheme(scheme))
	return scheme
}

func newTestMCO(binding string, enabled bool) *mcov1beta2.MultiClusterObservability {
	return newTestMCOFull(binding, enabled, binding, enabled)
}

func newTestMCOFull(nsBinding string, nsEnabled bool, virtBinding string, virtEnabled bool) *mcov1beta2.MultiClusterObservability {
	return &mcov1beta2.MultiClusterObservability{
		ObjectMeta: metav1.ObjectMeta{Name: "observability"},
		Spec: mcov1beta2.MultiClusterObservabilitySpec{
			Capabilities: &mcov1beta2.CapabilitiesSpec{
				Platform: &mcov1beta2.PlatformCapabilitiesSpec{
					Analytics: mcov1beta2.PlatformAnalyticsSpec{
						NamespaceRightSizingRecommendation: mcov1beta2.PlatformRightSizingRecommendationSpec{
							Enabled:          nsEnabled,
							NamespaceBinding: nsBinding,
						},
						VirtualizationRightSizingRecommendation: mcov1beta2.PlatformRightSizingRecommendationSpec{
							Enabled:          virtEnabled,
							NamespaceBinding: virtBinding,
						},
					},
				},
			},
		},
	}
}

func TestCreateRightSizingComponent_FeatureEnabled(t *testing.T) {
	scheme := setupTestScheme(t)

	mco := newTestMCO("custom-ns", true)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rsnamespace.ConfigMapName,
			Namespace: rsutility.DefaultNamespace,
		},
		Data: map[string]string{
			"config.yaml": `
				prometheusRuleConfig:
				namespaceFilterCriteria:
					inclusionCriteria: ["ns1"]
					exclusionCriteria: []
				labelFilterCriteria: []
				recommendationPercentage: 110
				placementConfiguration:
				predicates: []
				`,
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mco, configMap).
		Build()

	err := CreateRightSizingComponent(context.TODO(), client, mco)
	require.NoError(t, err)
}

func TestCreateRightSizingComponent_FeatureDisabled(t *testing.T) {
	scheme := setupTestScheme(t)

	mco := newTestMCO("", false)

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mco).
		Build()

	err := CreateRightSizingComponent(context.TODO(), client, mco)
	require.NoError(t, err)
}

// createRSResources creates all right-sizing resources for testing cleanup.
// policyNamespace is where Placements/Policies/PlacementBindings go (e.g. global-set).
// ConfigMaps always go in the observability namespace (config.GetDefaultNamespace()).
func createRSResources(t *testing.T, c client.Client, policyNamespace string) {
	t.Helper()
	ctx := context.TODO()
	configMapNs := mcoconfig.GetDefaultNamespace()

	resources := []client.Object{
		// Namespace RS resources
		&policyv1.PlacementBinding{ObjectMeta: metav1.ObjectMeta{Name: rsnamespace.PlacementBindingName, Namespace: policyNamespace}},
		&clusterv1beta1.Placement{ObjectMeta: metav1.ObjectMeta{Name: rsnamespace.PlacementName, Namespace: policyNamespace}},
		&policyv1.Policy{ObjectMeta: metav1.ObjectMeta{Name: rsnamespace.PrometheusRulePolicyName, Namespace: policyNamespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: rsnamespace.ConfigMapName, Namespace: configMapNs}},
		// Virtualization RS resources
		&policyv1.PlacementBinding{ObjectMeta: metav1.ObjectMeta{Name: rsvirtualization.PlacementBindingName, Namespace: policyNamespace}},
		&clusterv1beta1.Placement{ObjectMeta: metav1.ObjectMeta{Name: rsvirtualization.PlacementName, Namespace: policyNamespace}},
		&policyv1.Policy{ObjectMeta: metav1.ObjectMeta{Name: rsvirtualization.PrometheusRulePolicyName, Namespace: policyNamespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: rsvirtualization.ConfigMapName, Namespace: configMapNs}},
	}

	for _, r := range resources {
		require.NoError(t, c.Create(ctx, r), "failed to create %s/%s", r.GetNamespace(), r.GetName())
	}
}

// assertResourceNotFound verifies a resource no longer exists.
func assertResourceNotFound(t *testing.T, c client.Client, obj client.Object, key types.NamespacedName) {
	t.Helper()
	err := c.Get(context.TODO(), key, obj)
	assert.True(t, apierrors.IsNotFound(err), "expected NotFound for %s/%s, got: %v", key.Namespace, key.Name, err)
}

func TestCleanupRightSizingResources(t *testing.T) {
	scheme := setupTestScheme(t)
	globalSetNs := rsutility.DefaultNamespace
	configMapNs := mcoconfig.GetDefaultNamespace()

	mco := newTestMCOFull("", true, "", true)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mco).
		Build()

	// Pre-create all 8 RS resources
	createRSResources(t, c, globalSetNs)

	// Call cleanup
	err := CleanupRightSizingResources(context.TODO(), c, mco)
	require.NoError(t, err)

	// Verify all namespace RS resources are gone
	assertResourceNotFound(t, c, &policyv1.PlacementBinding{}, types.NamespacedName{Name: rsnamespace.PlacementBindingName, Namespace: globalSetNs})
	assertResourceNotFound(t, c, &clusterv1beta1.Placement{}, types.NamespacedName{Name: rsnamespace.PlacementName, Namespace: globalSetNs})
	assertResourceNotFound(t, c, &policyv1.Policy{}, types.NamespacedName{Name: rsnamespace.PrometheusRulePolicyName, Namespace: globalSetNs})
	assertResourceNotFound(t, c, &corev1.ConfigMap{}, types.NamespacedName{Name: rsnamespace.ConfigMapName, Namespace: configMapNs})

	// Verify all virtualization RS resources are gone
	assertResourceNotFound(t, c, &policyv1.PlacementBinding{}, types.NamespacedName{Name: rsvirtualization.PlacementBindingName, Namespace: globalSetNs})
	assertResourceNotFound(t, c, &clusterv1beta1.Placement{}, types.NamespacedName{Name: rsvirtualization.PlacementName, Namespace: globalSetNs})
	assertResourceNotFound(t, c, &policyv1.Policy{}, types.NamespacedName{Name: rsvirtualization.PrometheusRulePolicyName, Namespace: globalSetNs})
	assertResourceNotFound(t, c, &corev1.ConfigMap{}, types.NamespacedName{Name: rsvirtualization.ConfigMapName, Namespace: configMapNs})
}

func TestCleanupRightSizingResources_CustomNamespace(t *testing.T) {
	scheme := setupTestScheme(t)
	customNs := "custom-global-set"
	configMapNs := mcoconfig.GetDefaultNamespace()

	mco := newTestMCOFull(customNs, true, customNs, true)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mco).
		Build()

	// Pre-create all 8 RS resources in custom namespace (ConfigMaps still in observability namespace)
	createRSResources(t, c, customNs)

	// Call cleanup
	err := CleanupRightSizingResources(context.TODO(), c, mco)
	require.NoError(t, err)

	// Verify all resources are gone from custom namespace
	assertResourceNotFound(t, c, &clusterv1beta1.Placement{}, types.NamespacedName{Name: rsnamespace.PlacementName, Namespace: customNs})
	assertResourceNotFound(t, c, &clusterv1beta1.Placement{}, types.NamespacedName{Name: rsvirtualization.PlacementName, Namespace: customNs})
	assertResourceNotFound(t, c, &corev1.ConfigMap{}, types.NamespacedName{Name: rsnamespace.ConfigMapName, Namespace: configMapNs})
	assertResourceNotFound(t, c, &corev1.ConfigMap{}, types.NamespacedName{Name: rsvirtualization.ConfigMapName, Namespace: configMapNs})
}

func TestCleanupRightSizingResources_AlreadyDeleted(t *testing.T) {
	scheme := setupTestScheme(t)

	mco := newTestMCOFull("", true, "", true)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mco).
		Build()

	// Call cleanup without creating any resources — should succeed (idempotent)
	err := CleanupRightSizingResources(context.TODO(), c, mco)
	require.NoError(t, err)
}
