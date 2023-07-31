/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internalrequest

import (
	"context"
	"github.com/go-logr/logr"
	libhandler "github.com/operator-framework/operator-lib/handler"
	"github.com/redhat-appstudio/internal-services/api/v1alpha1"
	"github.com/redhat-appstudio/internal-services/loader"
	"github.com/redhat-appstudio/internal-services/tekton"
	"github.com/redhat-appstudio/operator-toolkit/controller"
	"github.com/redhat-appstudio/operator-toolkit/predicates"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Controller reconciles an InternalRequest object
type Controller struct {
	client         client.Client
	internalClient client.Client
	log            logr.Logger
}

// +kubebuilder:rbac:groups=appstudio.redhat.com,resources=internalrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=appstudio.redhat.com,resources=internalrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=appstudio.redhat.com,resources=internalrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups=appstudio.redhat.com,resources=internalservicesconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tekton.dev,resources=pipelines,verbs=get;list;watch
// +kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := c.log.WithValues("InternalRequest", req.NamespacedName)

	internalRequest := &v1alpha1.InternalRequest{}
	err := c.client.Get(ctx, req.NamespacedName, internalRequest)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	adapter := newAdapter(ctx, c.client, c.internalClient, internalRequest, loader.NewLoader(), logger)

	return controller.ReconcileHandler([]controller.Operation{
		adapter.EnsureRequestINotCompleted,
		adapter.EnsureConfigIsLoaded, // This operation sets the config in the adapter to be used in other operations.
		adapter.EnsureRequestIsAllowed,
		adapter.EnsurePipelineExists, // This operation sets the pipeline in the adapter to be used in other operations.
		adapter.EnsurePipelineRunIsCreated,
		adapter.EnsureStatusIsTracked,
		adapter.EnsurePipelineRunIsDeleted,
	})
}

// Register registers the controller with the passed manager and log. This controller monitors new InternalRequests and
// filters out status updates. It also watches for PipelineRuns created by this controller and owned by the
// InternalRequests so the owner gets reconciled on PipelineRun changes.
func (c *Controller) Register(mgr ctrl.Manager, log *logr.Logger, remoteCluster cluster.Cluster) error {
	c.client = mgr.GetClient()
	c.log = log.WithName("internalRequest")

	return ctrl.NewControllerManagedBy(mgr).
		For(
			&v1alpha1.InternalRequest{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}, predicates.IgnoreAllPredicate{}),
		).
		Watches(
			source.NewKindWithCache(&v1alpha1.InternalRequest{}, remoteCluster.GetCache()),
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}, predicates.NewObjectsPredicate{}),
		).
		Watches(&source.Kind{Type: &tektonv1beta1.PipelineRun{}}, &libhandler.EnqueueRequestForAnnotation{
			Type: schema.GroupKind{
				Kind:  "InternalRequest",
				Group: "appstudio.redhat.com",
			},
		}, builder.WithPredicates(tekton.InternalRequestPipelineRunSucceededPredicate())).
		Complete(c)
}
