package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	argowfv1alpha1 "github.com/argoproj/argo-workflows/v4/pkg/apis/workflow/v1alpha1"

	workflowv1alpha1 "github.com/dipen1510/marketplace-workflow-platform/controller/api/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MarketplaceWorkflowReconciler reconciles MarketplaceWorkflow resources.
type MarketplaceWorkflowReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// RBAC for MarketplaceWorkflow.

// +kubebuilder:rbac:groups=workflow.marketplace.example.com,resources=marketplaceworkflows,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workflow.marketplace.example.com,resources=marketplaceworkflows/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workflow.marketplace.example.com,resources=marketplaceworkflows/finalizers,verbs=update

// We need to read WorkflowTemplates.

// +kubebuilder:rbac:groups=argoproj.io,resources=workflowtemplates,verbs=get;list;watch

// We fully manage CronWorkflows.

// +kubebuilder:rbac:groups=argoproj.io,resources=cronworkflows,verbs=get;list;watch;create;update;patch;delete

func (r *MarketplaceWorkflowReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	// ---------------------------------------------------------
	// 1. Fetch MarketplaceWorkflow
	// ---------------------------------------------------------

	var marketplaceWorkflow workflowv1alpha1.MarketplaceWorkflow

	if err := r.Get(
		ctx,
		req.NamespacedName,
		&marketplaceWorkflow,
	); err != nil {

		return ctrl.Result{},
			client.IgnoreNotFound(err)
	}

	logger.Info(
		"reconciling MarketplaceWorkflow",
		"name", marketplaceWorkflow.Name,
		"namespace", marketplaceWorkflow.Namespace,
	)

	// ---------------------------------------------------------
	// 2. Verify referenced Argo WorkflowTemplate exists
	// ---------------------------------------------------------

	templateName :=
		marketplaceWorkflow.Spec.WorkflowTemplateRef.Name

	var workflowTemplate argowfv1alpha1.WorkflowTemplate

	err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      templateName,
			Namespace: marketplaceWorkflow.Namespace,
		},
		&workflowTemplate,
	)

	if err != nil {

		if apierrors.IsNotFound(err) {

			message := fmt.Sprintf(
				"WorkflowTemplate %q does not exist",
				templateName,
			)

			logger.Info(message)

			if statusErr := r.markNotReady(
				ctx,
				&marketplaceWorkflow,
				"WorkflowTemplateNotFound",
				message,
			); statusErr != nil {

				return ctrl.Result{}, statusErr
			}

			// We don't own WorkflowTemplate, so for V1 periodically
			// check whether somebody creates it.
			return ctrl.Result{
				RequeueAfter: 30 * time.Second,
			}, nil
		}

		return ctrl.Result{}, err
	}

	// ---------------------------------------------------------
	// 3. Desired child Argo CronWorkflow
	// ---------------------------------------------------------

	cronWorkflow := &argowfv1alpha1.CronWorkflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      marketplaceWorkflow.Name,
			Namespace: marketplaceWorkflow.Namespace,
		},
	}

	// ---------------------------------------------------------
	// 4. Create OR update CronWorkflow
	// ---------------------------------------------------------

	operationResult, err :=
		controllerutil.CreateOrUpdate(
			ctx,
			r.Client,
			cronWorkflow,
			func() error {

				// IMPORTANT:
				// Never modify an existing CronWorkflow
				// unless this MarketplaceWorkflow owns it.
				if err := ensureCronWorkflowOwnership(
					&marketplaceWorkflow,
					cronWorkflow,
				); err != nil {
					return err
				}

				if cronWorkflow.Labels == nil {
					cronWorkflow.Labels =
						make(map[string]string)
				}

				cronWorkflow.Labels["app.kubernetes.io/managed-by"] = "marketplace-workflow-controller"

				cronWorkflow.Labels["marketplace.example.com/workflow"] = marketplaceWorkflow.Name

				// -----------------------------------------
				// Scheduling
				// -----------------------------------------

				cronWorkflow.Spec.Schedules =
					append(
						[]string(nil),
						marketplaceWorkflow.Spec.Schedules...,
					)

				cronWorkflow.Spec.Timezone =
					marketplaceWorkflow.Spec.Timezone

				cronWorkflow.Spec.Suspend =
					marketplaceWorkflow.Spec.Suspend

				cronWorkflow.Spec.ConcurrencyPolicy =
					argowfv1alpha1.ConcurrencyPolicy(
						marketplaceWorkflow.Spec.
							ConcurrencyPolicy,
					)

				cronWorkflow.Spec.StartingDeadlineSeconds =
					marketplaceWorkflow.Spec.
						StartingDeadlineSeconds

				cronWorkflow.Spec.SuccessfulJobsHistoryLimit =
					marketplaceWorkflow.Spec.
						SuccessfulRunsHistoryLimit

				cronWorkflow.Spec.FailedJobsHistoryLimit =
					marketplaceWorkflow.Spec.
						FailedRunsHistoryLimit

				// -----------------------------------------
				// Parameters
				// -----------------------------------------

				parameters :=
					make(
						[]argowfv1alpha1.Parameter,
						0,
						len(
							marketplaceWorkflow.
								Spec.Parameters,
						),
					)

				for _, parameter := range marketplaceWorkflow.Spec.Parameters {

					parameters =
						append(
							parameters,
							argowfv1alpha1.Parameter{
								Name: parameter.Name,

								Value: argowfv1alpha1.
									AnyStringPtr(
										parameter.Value,
									),
							},
						)
				}

				// -----------------------------------------
				// Argo WorkflowTemplate reference
				// -----------------------------------------

				cronWorkflow.Spec.WorkflowSpec =
					argowfv1alpha1.WorkflowSpec{

						WorkflowTemplateRef: &argowfv1alpha1.
							WorkflowTemplateRef{

							Name: marketplaceWorkflow.
								Spec.
								WorkflowTemplateRef.
								Name,
						},

						Arguments: argowfv1alpha1.Arguments{
							Parameters: parameters,
						},
					}

				// -----------------------------------------
				// Ownership
				// -----------------------------------------

				return controllerutil.
					SetControllerReference(
						&marketplaceWorkflow,
						cronWorkflow,
						r.Scheme,
					)
			},
		)

	if err != nil {

		if errors.Is(err, ErrCronWorkflowConflict) {

			message := fmt.Sprintf(
				"CronWorkflow %q already exists and is not owned by MarketplaceWorkflow %q",
				cronWorkflow.Name,
				marketplaceWorkflow.Name,
			)

			logger.Info(
				"cronworkflow ownership conflict",
				"cronWorkflow",
				cronWorkflow.Name,
			)

			if statusErr := r.markNotReady(
				ctx,
				&marketplaceWorkflow,
				"CronWorkflowConflict",
				message,
			); statusErr != nil {
				return ctrl.Result{}, statusErr
			}

			return ctrl.Result{}, nil
		}

		_ = r.markNotReady(
			ctx,
			&marketplaceWorkflow,
			"CronWorkflowReconcileFailed",
			err.Error(),
		)

		return ctrl.Result{}, err
	}

	logger.Info(
		"reconciled Argo CronWorkflow",
		"cronWorkflow", cronWorkflow.Name,
		"operation", operationResult,
	)

	// ---------------------------------------------------------
	// 5. Copy Argo state → MarketplaceWorkflow status
	// ---------------------------------------------------------

	if err := r.syncStatus(
		ctx,
		&marketplaceWorkflow,
		cronWorkflow,
	); err != nil {

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MarketplaceWorkflowReconciler) markNotReady(
	ctx context.Context,
	marketplaceWorkflow *workflowv1alpha1.MarketplaceWorkflow,
	reason string,
	message string,
) error {

	before := marketplaceWorkflow.DeepCopy()

	marketplaceWorkflow.Status.Phase =
		workflowv1alpha1.MarketplaceWorkflowNotReady

	marketplaceWorkflow.Status.ObservedGeneration =
		marketplaceWorkflow.Generation

	meta.SetStatusCondition(
		&marketplaceWorkflow.Status.Conditions,
		metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: marketplaceWorkflow.Generation,
		},
	)

	if reflect.DeepEqual(
		before.Status,
		marketplaceWorkflow.Status,
	) {
		return nil
	}

	return r.Status().Patch(
		ctx,
		marketplaceWorkflow,
		client.MergeFrom(before),
	)
}

func (r *MarketplaceWorkflowReconciler) syncStatus(
	ctx context.Context,
	marketplaceWorkflow *workflowv1alpha1.MarketplaceWorkflow,
	cronWorkflow *argowfv1alpha1.CronWorkflow,
) error {

	before := marketplaceWorkflow.DeepCopy()

	marketplaceWorkflow.Status.Phase =
		workflowv1alpha1.MarketplaceWorkflowReady

	marketplaceWorkflow.Status.CronWorkflowName =
		cronWorkflow.Name

	marketplaceWorkflow.Status.ActiveRuns =
		int32(len(cronWorkflow.Status.Active))

	marketplaceWorkflow.Status.LastScheduledTime =
		cronWorkflow.Status.LastScheduledTime

	marketplaceWorkflow.Status.SucceededRuns =
		cronWorkflow.Status.Succeeded

	marketplaceWorkflow.Status.FailedRuns =
		cronWorkflow.Status.Failed

	marketplaceWorkflow.Status.ObservedGeneration =
		marketplaceWorkflow.Generation

	meta.SetStatusCondition(
		&marketplaceWorkflow.Status.Conditions,
		metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "CronWorkflowReady",
			Message:            "Argo CronWorkflow is reconciled",
			ObservedGeneration: marketplaceWorkflow.Generation,
		},
	)

	if reflect.DeepEqual(
		before.Status,
		marketplaceWorkflow.Status,
	) {
		return nil
	}

	return r.Status().Patch(
		ctx,
		marketplaceWorkflow,
		client.MergeFrom(before),
	)
}

const workflowTemplateRefField = "spec.workflowTemplateRef.name"

func (r *MarketplaceWorkflowReconciler) SetupWithManager(
	mgr ctrl.Manager,
) error {

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&workflowv1alpha1.MarketplaceWorkflow{},
		workflowTemplateRefField,
		func(rawObj client.Object) []string {

			mw :=
				rawObj.(*workflowv1alpha1.MarketplaceWorkflow)

			if mw.Spec.WorkflowTemplateRef.Name == "" {
				return nil
			}

			return []string{
				mw.Spec.WorkflowTemplateRef.Name,
			}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(
			&workflowv1alpha1.MarketplaceWorkflow{},
		).
		Owns(
			&argowfv1alpha1.CronWorkflow{},
		).
		Watches(
			&argowfv1alpha1.WorkflowTemplate{},
			handler.EnqueueRequestsFromMapFunc(
				r.requestsForWorkflowTemplate,
			),
		).
		Named("marketplaceworkflow").
		Complete(r)
}

func (r *MarketplaceWorkflowReconciler) requestsForWorkflowTemplate(
	ctx context.Context,
	obj client.Object,
) []reconcile.Request {

	var workflows workflowv1alpha1.MarketplaceWorkflowList

	err := r.List(
		ctx,
		&workflows,
		client.InNamespace(obj.GetNamespace()),
		client.MatchingFields{
			workflowTemplateRefField: obj.GetName(),
		},
	)

	if err != nil {
		return nil
	}

	requests :=
		make(
			[]reconcile.Request,
			0,
			len(workflows.Items),
		)

	for _, workflow := range workflows.Items {

		requests = append(
			requests,
			reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: workflow.Name,

					Namespace: workflow.Namespace,
				},
			},
		)
	}

	return requests
}

var ErrCronWorkflowConflict = errors.New(
	"cronworkflow already exists and is not controlled by MarketplaceWorkflow",
)

func ensureCronWorkflowOwnership(
	mw *workflowv1alpha1.MarketplaceWorkflow,
	cron *argowfv1alpha1.CronWorkflow,
) error {

	// No UID means CreateOrUpdate is constructing a new object.
	if cron.UID == "" {
		return nil
	}

	controllerRef := metav1.GetControllerOf(cron)

	// Existing but nobody owns it.
	// Do not automatically adopt it.
	if controllerRef == nil {
		return ErrCronWorkflowConflict
	}

	// Somebody else controls it.
	if controllerRef.UID != mw.UID {
		return ErrCronWorkflowConflict
	}

	return nil
}
