// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package analytics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	mcov1beta2 "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/api/v1beta2"
	rightsizingctrl "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/analytics/rightsizing"
	mcoctrl "github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/controllers/multiclusterobservability"
	"github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/pkg/config"
	"github.com/stolostron/multicluster-observability-operator/operators/multiclusterobservability/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var log = logf.Log.WithName("controller_rightsizing")

var mcoGVK = mcov1beta2.GroupVersion.WithKind("MultiClusterObservability")

// AnalyticsReconciler reconciles a MultiClusterObservability object
type AnalyticsReconciler struct {
	Client   client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=observability.open-cluster-management.io,resources=multiclusterobservabilities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=observability.open-cluster-management.io,resources=multiclusterobservabilities/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=observability.open-cluster-management.io,resources=multiclusterobservabilities/finalizers,verbs=update

func (r *AnalyticsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// TODO: Future enhancement - Add status subresource to track right-sizing state
	// This would allow users to see current mode (MCO Policy vs MCOA ManifestWork)
	// and configuration details via: kubectl get mco -o jsonpath='{.status.rightSizing}'

	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling RightSizing")

	// Fetch the MultiClusterObservability instance
	mcoList := &mcov1beta2.MultiClusterObservabilityList{}
	err := r.Client.List(ctx, mcoList)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list MultiClusterObservability custom resources: %w", err)
	}
	if len(mcoList.Items) == 0 {
		reqLogger.Info("no MultiClusterObservability CR exists, nothing to do")
		return ctrl.Result{}, nil
	}

	instance := mcoList.Items[0].DeepCopy()

	// Do not reconcile objects if this instance of mch is labeled "paused"
	if config.IsPaused(instance.GetAnnotations()) {
		reqLogger.Info("MCO reconciliation is paused. Nothing more to do.")
		return ctrl.Result{}, nil
	}

	// Ensure defaults are set/persisted for analytics right-sizing
	instance, err = r.ensureRightSizingDefaults(ctx, instance, reqLogger)
	if err != nil {
		return ctrl.Result{}, err
	}

	// ═══════════════════════════════════════════════════════════════════
	// MIGRATION GATE: Check if MCOA should handle right-sizing
	// ═══════════════════════════════════════════════════════════════════

	// Check 1: Is platform metrics enabled? (MCOA prerequisite)
	// MCOA is only deployed when platform metrics is enabled
	if isPlatformMetricsEnabled(instance) {
		// Check 2: Is MCOA capable of handling right-sizing?
		mcoaCapable, err := util.IsMCOARightSizingCapable(ctx, r.Client)
		if err != nil {
			reqLogger.Error(err, "Failed to check MCOA right-sizing capability, proceeding with Policy-based approach")
			mcoaCapable = false
		}

		if mcoaCapable {
			reqLogger.Info("MCOA is right-sizing capable, delegating to MCOA")
			// Cleanup Policy resources but keep ConfigMaps
			rightsizingctrl.CleanupPolicyResourcesForDelegation(ctx, r.Client, instance)
			// Note: Do NOT sync disabled state to AddOnDeploymentConfig
			// MCOA will auto-enable when keys are not set

			// Record event for observability
			if r.Recorder != nil {
				r.Recorder.Event(instance, corev1.EventTypeNormal, "RightSizingDelegated",
					"Right-sizing management delegated to MCOA (ClusterManagementAddOn has capability annotation)")
			}
			return ctrl.Result{}, nil
		}
	} else {
		reqLogger.V(1).Info("Platform metrics not enabled, MCO will manage right-sizing via Policy")
	}

	// ═══════════════════════════════════════════════════════════════════
	// MCO Mode: Create Policy resources (current GA behavior)
	// ═══════════════════════════════════════════════════════════════════
	reqLogger.V(1).Info("MCO managing right-sizing via Policy")
	err = rightsizingctrl.CreateRightSizingComponent(ctx, r.Client, instance)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create rightsizing component: %w", err)
	}

	// When MCO manages right-sizing, sync disabled state to AddOnDeploymentConfig
	// This tells MCOA to NOT deploy PrometheusRules via ManifestWork
	if err := r.syncDisabledStateToADC(ctx, reqLogger); err != nil {
		reqLogger.Error(err, "Failed to sync disabled state to AddOnDeploymentConfig")
		// Don't fail the reconcile, MCO can still manage via Policy
	}

	return ctrl.Result{}, nil
}

// ensureRightSizingDefaults persists default right-sizing flags when absent and returns the (possibly updated) instance.
func (r *AnalyticsReconciler) ensureRightSizingDefaults(ctx context.Context, instance *mcov1beta2.MultiClusterObservability, reqLogger logr.Logger) (*mcov1beta2.MultiClusterObservability, error) {
	// Default-enable analytics right-sizing flags ONLY when absent on fresh installs.
	// Persist defaults back to the MCO spec so users can later override to true/false explicitly.
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(mcoGVK)
	key := types.NamespacedName{Name: instance.GetName()}

	if err := r.Client.Get(ctx, key, u); err == nil {
		// Check if the fields already exist
		nsEnabled, nsFound, _ := unstructured.NestedBool(u.Object,
			"spec", "capabilities", "platform", "analytics", "namespaceRightSizingRecommendation", "enabled")
		virtEnabled, virtFound, _ := unstructured.NestedBool(u.Object,
			"spec", "capabilities", "platform", "analytics", "virtualizationRightSizingRecommendation", "enabled")

		// Only patch if at least one field is missing
		if !nsFound || !virtFound {
			// Build a minimal patch that only contains the analytics fields we want to set.
			// Use typed locals to avoid chained type assertions (which can panic if the shape changes).
			// Set true if not present else preserve existing value
			analytics := map[string]any{
				"namespaceRightSizingRecommendation":      map[string]any{"enabled": !nsFound || nsEnabled},
				"virtualizationRightSizingRecommendation": map[string]any{"enabled": !virtFound || virtEnabled},
			}
			patchData := map[string]any{
				"spec": map[string]any{
					"capabilities": map[string]any{
						"platform": map[string]any{
							"analytics": analytics,
						},
					},
				},
			}

			patchBytes, err := json.Marshal(patchData)
			if err != nil {
				return instance, fmt.Errorf("failed to marshal patch data: %w", err)
			}

			// Use MergePatch to only update the specific fields without affecting others
			if err := r.Client.Patch(ctx, u, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
				return instance, fmt.Errorf("failed to persist default analytics right-sizing flags: %w", err)
			}
			reqLogger.Info("Defaulted analytics right-sizing flags to true (fresh install)")

			// refresh typed instance so downstream logic sees updated flags
			refreshed := &mcov1beta2.MultiClusterObservability{}
			if err := r.Client.Get(ctx, key, refreshed); err != nil {
				reqLogger.Error(err, "Failed to refresh MCO after patching defaults, using stale instance")
			} else {
				instance = refreshed.DeepCopy()
			}
		}
	}
	return instance, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AnalyticsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := mgr.GetClient()
	ctx := context.Background()

	mcoPred := mcoctrl.GetMCOPredicateFunc()
	cmNamespaceRSPred := rightsizingctrl.GetNamespaceRSConfigMapPredicateFunc(ctx, c)
	cmVirtualizationRSPred := rightsizingctrl.GetVirtualizationRSConfigMapPredicateFunc(ctx, c)
	cmaPred := getMCOAClusterManagementAddonPredicateFunc()

	return ctrl.NewControllerManagedBy(mgr).
		Named("rightsizing").
		For(&mcov1beta2.MultiClusterObservability{}, builder.WithPredicates(mcoPred)).
		Watches(&corev1.ConfigMap{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(cmNamespaceRSPred)).
		Watches(&corev1.ConfigMap{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(cmVirtualizationRSPred)).
		// Use EnqueueRequestsFromMapFunc for CMA watch since Reconcile lists all MCOs anyway.
		// The actual request name doesn't matter - we use a consistent trigger name.
		Watches(&addonv1alpha1.ClusterManagementAddOn{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []ctrl.Request {
				// Enqueue a request that will cause MCO List to be called.
				// The actual request name doesn't matter since Reconcile lists all MCOs.
				return []ctrl.Request{{
					NamespacedName: types.NamespacedName{Name: "cma-change-trigger"},
				}}
			}),
			builder.WithPredicates(cmaPred)).
		Complete(r)
}

// getMCOAClusterManagementAddonPredicateFunc returns a predicate that filters for MCOA ClusterManagementAddOn changes
func getMCOAClusterManagementAddonPredicateFunc() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == util.MCOAClusterManagementAddOnName
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew.GetName() != util.MCOAClusterManagementAddOnName {
				return false
			}
			// Only trigger if annotations changed (specifically the right-sizing-capable annotation)
			oldAnnotations := e.ObjectOld.GetAnnotations()
			newAnnotations := e.ObjectNew.GetAnnotations()
			oldValue := ""
			newValue := ""
			if oldAnnotations != nil {
				oldValue = oldAnnotations[util.RightSizingCapableAnnotation]
			}
			if newAnnotations != nil {
				newValue = newAnnotations[util.RightSizingCapableAnnotation]
			}
			return oldValue != newValue
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// When MCOA ClusterManagementAddOn is deleted, trigger reconcile.
			// IsMCOARightSizingCapable() will return false, causing MCO to take over via Policy.
			return e.Object.GetName() == util.MCOAClusterManagementAddOnName
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// isPlatformMetricsEnabled checks if platform metrics is enabled in the MCO CRD.
// MCOA is only deployed when platform metrics is enabled, so this is a prerequisite
// for delegating right-sizing management to MCOA.
func isPlatformMetricsEnabled(mco *mcov1beta2.MultiClusterObservability) bool {
	if mco.Spec.Capabilities == nil {
		return false
	}
	if mco.Spec.Capabilities.Platform == nil {
		return false
	}
	return mco.Spec.Capabilities.Platform.Metrics.Default.Enabled
}

// syncDisabledStateToADC syncs disabled state to AddOnDeploymentConfig when MCO manages right-sizing.
// This tells MCOA to NOT deploy PrometheusRules via ManifestWork.
// Uses MCOA's key names: platformNamespaceRightSizing, platformVirtualizationRightSizing with value "disabled"
func (r *AnalyticsReconciler) syncDisabledStateToADC(ctx context.Context, reqLogger logr.Logger) error {
	const (
		keyPlatformNamespaceRightSizing      = "platformNamespaceRightSizing"
		keyPlatformVirtualizationRightSizing = "platformVirtualizationRightSizing"
		valueDisabled                        = "disabled"
	)

	adc := &addonv1alpha1.AddOnDeploymentConfig{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      util.MCOAClusterManagementAddOnName,
		Namespace: config.GetDefaultNamespace(),
	}, adc)
	if err != nil {
		// ADC doesn't exist - nothing to sync
		return nil
	}

	// Single-pass: find indices and track if update needed
	nsIdx, virtIdx := -1, -1
	needsUpdate := false

	for i, cv := range adc.Spec.CustomizedVariables {
		switch cv.Name {
		case keyPlatformNamespaceRightSizing:
			nsIdx = i
			if cv.Value != valueDisabled {
				adc.Spec.CustomizedVariables[i].Value = valueDisabled
				needsUpdate = true
			}
		case keyPlatformVirtualizationRightSizing:
			virtIdx = i
			if cv.Value != valueDisabled {
				adc.Spec.CustomizedVariables[i].Value = valueDisabled
				needsUpdate = true
			}
		}
	}

	// Append if not found
	if nsIdx == -1 {
		adc.Spec.CustomizedVariables = append(adc.Spec.CustomizedVariables,
			addonv1alpha1.CustomizedVariable{Name: keyPlatformNamespaceRightSizing, Value: valueDisabled})
		needsUpdate = true
	}
	if virtIdx == -1 {
		adc.Spec.CustomizedVariables = append(adc.Spec.CustomizedVariables,
			addonv1alpha1.CustomizedVariable{Name: keyPlatformVirtualizationRightSizing, Value: valueDisabled})
		needsUpdate = true
	}

	if needsUpdate {
		reqLogger.V(1).Info("rs - syncing disabled state to AddOnDeploymentConfig (MCO takes over)")
		if err := r.Client.Update(ctx, adc); err != nil {
			return fmt.Errorf("failed to update AddOnDeploymentConfig: %w", err)
		}
	}

	return nil
}
