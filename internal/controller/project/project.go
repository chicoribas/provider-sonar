/*
Copyright 2022 The Crossplane Authors.

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

package project

import (
	"context"
	"fmt"
	"log"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-sonar/apis/project/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-sonar/apis/v1alpha1"
	"github.com/crossplane/provider-sonar/internal/clients/sonar"
	"github.com/crossplane/provider-sonar/internal/controller/features"
)

const (
	errNotProject   = "managed resource is not a Project custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

// Setup adds a controller that reconciles Project managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ProjectGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ProjectGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:        mgr.GetClient(),
			usage:       resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newClientFn: sonar.NewProjectClient}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.Project{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube        client.Client
	usage       resource.Tracker
	newClientFn func(options sonar.SonarApiOptions) sonar.ProjectClient
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Project)
	if !ok {
		return nil, errors.New(errNotProject)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cd := pc.Spec.Credentials
	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}
	fmt.Println(string(data))

	svc := c.newClientFn(sonar.SonarApiOptions{Key: string(data)})
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{projectClient: svc}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	projectClient sonar.ProjectClient
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Project)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotProject)
	}

	// These fmt statements should be removed in the real implementation.
	fmt.Printf("Observing: %+v", cr)

	project, err := c.projectClient.GetByProjectKey(ctx, cr.Spec.ForProvider.Organization, cr.Spec.ForProvider.Key)

	if err != nil {
		if errors.Is(err, sonar.ErrProjectNotFound) {
			return managed.ExternalObservation{
				ResourceExists:          false,
				ResourceLateInitialized: false,
			}, nil
		}
		return managed.ExternalObservation{}, err
	}

	fmt.Println("\n\nproject.Visibility:" + project.Visibility)
	fmt.Println("cr.Spec.ForProvider.Visibility:" + cr.Spec.ForProvider.Visibility + "\n\n")

	if project.Visibility != cr.Spec.ForProvider.Visibility {
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, nil
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil

	// return managed.ExternalObservation{
	// 	// Return false when the external resource does not exist. This lets
	// 	// the managed resource reconciler know that it needs to call Create to
	// 	// (re)create the resource, or that it has successfully been deleted.
	// 	ResourceExists: true,

	// 	// Return false when the external resource exists, but it not up to date
	// 	// with the desired managed resource state. This lets the managed
	// 	// resource reconciler know that it needs to call Update.
	// 	ResourceUpToDate: true,

	// 	// Return any details that may be required to connect to the external
	// 	// resource. These will be stored as the connection secret.
	// 	ConnectionDetails: managed.ConnectionDetails{},
	// }, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Project)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotProject)
	}

	fmt.Printf("Creating: %+v", cr)

	_, err := c.projectClient.Create(ctx, cr.Spec.ForProvider.Organization, cr.GetObjectMeta().GetName(), cr.Spec.ForProvider.Key, cr.Spec.ForProvider.Visibility)

	if err != nil {
		log.Fatal(err)
	}

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Project)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotProject)
	}

	fmt.Printf("Updating: %+v", cr)

	err := c.projectClient.UpdateVisibility(ctx, cr.Spec.ForProvider.Key, cr.Spec.ForProvider.Visibility)
	if err != nil {
		log.Fatal(err)
	}

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Project)
	if !ok {
		return errors.New(errNotProject)
	}

	fmt.Printf("Deleting: %+v", cr)

	err := c.projectClient.Delete(ctx, cr.Spec.ForProvider.Key)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}
