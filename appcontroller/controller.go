/*
Copyright 2017 The Kubernetes Authors.

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

package main

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v13 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	v15 "k8s.io/client-go/informers/core/v1"
	v12 "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	v14 "k8s.io/client-go/listers/core/v1"
	v1 "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"time"

	appv1 "github.com/2456868764/operator/appcontroller/pkg/apis/appcontroller/v1"
	clientset "github.com/2456868764/operator/appcontroller/pkg/generated/clientset/versioned"
	appscheme "github.com/2456868764/operator/appcontroller/pkg/generated/clientset/versioned/scheme"
	informers "github.com/2456868764/operator/appcontroller/pkg/generated/informers/externalversions/appcontroller/v1"
	listers "github.com/2456868764/operator/appcontroller/pkg/generated/listers/appcontroller/v1"
)

const controllerAgentName = "appcontroller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a App is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a App fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by App"
	// MessageResourceSynced is the message used for an Event fired when a App
	// is synced successfully
	MessageResourceSynced = "App synced successfully"
)

// Controller is the controller implementation for App resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// appclientset is a clientset for our own API group
	appclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	ingressLister     v1.IngressLister
	ingressSynced     cache.InformerSynced
	serviceLister     v14.ServiceLister
	serviceSynced     cache.InformerSynced
	appsLister        listers.AppLister
	appsSynced        cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	appclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	serviceInformer v15.ServiceInformer,
	appInformer informers.AppInformer,
	ingressInformer v12.IngressInformer) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(appscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:     kubeclientset,
		appclientset:      appclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		appsLister:        appInformer.Lister(),
		appsSynced:        appInformer.Informer().HasSynced,
		ingressLister:     ingressInformer.Lister(),
		ingressSynced:     ingressInformer.Informer().HasSynced,
		serviceLister:     serviceInformer.Lister(),
		serviceSynced:     serviceInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Apps"),
		recorder:          recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when App resources change
	appInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueApp,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueApp(new)
		},
	})
	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a App resource then the handler will enqueue that App resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md

	//deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc: controller.handleObject,
	//	UpdateFunc: func(old, new interface{}) {
	//		newDepl := new.(*appsv1.Deployment)
	//		oldDepl := old.(*appsv1.Deployment)
	//		if newDepl.ResourceVersion == oldDepl.ResourceVersion {
	//			// Periodic resync will send update events for all known Deployments.
	//			// Two different versions of the same Deployment will always have different RVs.
	//			return
	//		}
	//		controller.handleObject(new)
	//	},
	//	DeleteFunc: controller.handleObject,
	//})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting App controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.appsSynced, c.ingressSynced, c.serviceSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process App resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)

		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// App resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the App resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the App resource with this namespace/name
	app, err := c.appsLister.Apps(namespace).Get(name)
	if err != nil {
		// The App resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	err = c.syncDeployment(key, app)
	if err != nil {
		return err
	}

	err = c.syncService(key, app)
	if err != nil {
		return err
	}

	err = c.syncIngress(key, app)
	if err != nil {
		return err
	}

	c.recorder.Event(app, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) syncDeployment(key string, app *appv1.App) error {
	deploymentName := app.Spec.Deployment.Name
	if deploymentName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
		return nil
	}

	// Get the deployment with the name specified in App.spec
	deployment, err := c.deploymentsLister.Deployments(app.Namespace).Get(deploymentName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(app.Namespace).Create(context.TODO(), newDeployment(app), metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this App resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(deployment, app) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(app, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf("%s", msg)
	}

	// If this number of the replicas on the App resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.
	if app.Spec.Deployment.Replicas != nil && *app.Spec.Deployment.Replicas != *deployment.Spec.Replicas {
		klog.V(4).Infof("App %s replicas: %d, deployment replicas: %d", app.Name, *app.Spec.Deployment.Replicas, *deployment.Spec.Replicas)
		deployment, err = c.kubeclientset.AppsV1().Deployments(app.Namespace).Update(context.TODO(), newDeployment(app), metav1.UpdateOptions{})
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the App resource to reflect the
	// current state of the world
	//err = c.updateAppStatus(app, deployment)
	//if err != nil {
	//	return err
	//}

	return nil
}

func (c *Controller) syncService(key string, app *appv1.App) error {
	serviceName := app.Spec.Service.Name
	if serviceName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: service name must be specified", key))
		return nil
	}

	// Get the deployment with the name specified in App.spec
	service, err := c.serviceLister.Services(app.Namespace).Get(serviceName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		service, err = c.kubeclientset.CoreV1().Services(app.Namespace).Create(context.TODO(), newService(app), metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this App resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(service, app) {
		msg := fmt.Sprintf(MessageResourceExists, service.Name)
		c.recorder.Event(app, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf("%s", msg)
	}

	return nil
}

func (c *Controller) syncIngress(key string, app *appv1.App) error {
	ingressName := app.Spec.Ingress.Name
	ingressHostname := app.Spec.Ingress.Hostname
	if ingressName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: ingress name must be specified", key))
		return nil
	}

	if ingressHostname == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: ingress hostname must be specified", key))
		return nil
	}

	// Get the deployment with the name specified in App.spec
	ingress, err := c.ingressLister.Ingresses(app.Namespace).Get(ingressName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		ingress, err = c.kubeclientset.NetworkingV1().Ingresses(app.Namespace).Create(context.TODO(), newIngress(app), metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this App resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(ingress, app) {
		msg := fmt.Sprintf(MessageResourceExists, ingress.Name)
		c.recorder.Event(app, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf("%s", msg)
	}

	return nil
}

func (c *Controller) updateAppStatus(app *appv1.App, deployment *appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	appCopy := app.DeepCopy()
	appCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the App resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.appclientset.AppcontrollerV1().Apps(app.Namespace).UpdateStatus(context.TODO(), appCopy, metav1.UpdateOptions{})
	return err
}

// enqueueApp takes a App resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than App.
func (c *Controller) enqueueApp(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the App resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that App resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a App, we should not do anything more
		// with it.
		if ownerRef.Kind != "App" {
			return
		}

		app, err := c.appsLister.Apps(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s/%s' of foo '%s'", object.GetNamespace(), object.GetName(), ownerRef.Name)
			return
		}

		c.enqueueApp(app)
		return
	}
}

// newDeployment creates a new Deployment for a App resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the App resource that 'owns' it.
func newDeployment(app *appv1.App) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "app",
		"controller": app.Name,
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Spec.Deployment.Name,
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, appv1.SchemeGroupVersion.WithKind("App")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: app.Spec.Deployment.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  app.Spec.Deployment.Name,
							Image: app.Spec.Deployment.Image,
						},
					},
				},
			},
		},
	}
}

func newIngress(app *appv1.App) *v13.Ingress {
	pathType := v13.PathTypePrefix
	return &v13.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Spec.Ingress.Name,
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, appv1.SchemeGroupVersion.WithKind("App")),
			},
		},
		Spec: v13.IngressSpec{
			Rules: []v13.IngressRule{
				{
					Host: app.Spec.Ingress.Hostname,
					IngressRuleValue: v13.IngressRuleValue{
						HTTP: &v13.HTTPIngressRuleValue{
							Paths: []v13.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: v13.IngressBackend{
										Service: &v13.IngressServiceBackend{
											Name: app.Spec.Service.Name,
											Port: v13.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

}
func newService(app *appv1.App) *corev1.Service {
	labels := map[string]string{
		"app":        "app-deployment",
		"controller": app.Name,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Spec.Service.Name,
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, appv1.SchemeGroupVersion.WithKind("App")),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.IntOrString{IntVal: 80},
				},
			},
		},
	}
}
