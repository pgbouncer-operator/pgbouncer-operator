package pgbouncer

import (
	"context"
	"reflect"

	pgbounceroperatorv1alpha1 "github.com/pgbouncer-operator/pgbouncer-operator/pkg/apis/pgbounceroperator/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_pgbouncer")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new PgBouncer Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePgBouncer{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("pgbouncer-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PgBouncer
	err = c.Watch(&source.Kind{Type: &pgbounceroperatorv1alpha1.PgBouncer{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployments and requeue the owner PgBouncer
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &pgbounceroperatorv1alpha1.PgBouncer{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Services and requeue the owner PgBouncer
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &pgbounceroperatorv1alpha1.PgBouncer{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePgBouncer implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePgBouncer{}

// ReconcilePgBouncer reconciles a PgBouncer object
type ReconcilePgBouncer struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a PgBouncer object and makes changes based on the state read
// and what is in the PgBouncer.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePgBouncer) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling PgBouncer.")

	// Fetch the PgBouncer instance
	pgbouncer := &pgbounceroperatorv1alpha1.PgBouncer{}
	err := r.client.Get(context.TODO(), request.NamespacedName, pgbouncer)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("PgBouncer resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get PgBouncer.")
		return reconcile.Result{}, err
	}

	// Check if the Deployment already exists, if not create a new one
	deployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pgbouncer.Name, Namespace: pgbouncer.Namespace}, deployment)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Deployment
		dep := r.deploymentForPgBouncer(pgbouncer)
		reqLogger.Info("Creating a new Deployment.", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment.", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		// Deployment created successfully - return and requeue
		// NOTE: that the requeue is made with the purpose to provide the deployment object for the next step to ensure the deployment size is the same as the spec.
		// Also, you could GET the deployment object again instead of requeue if you wish. See more over it here: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/reconcile#Reconciler
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment.")
		return reconcile.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := pgbouncer.Spec.Size
	if *deployment.Spec.Replicas != size {
		deployment.Spec.Replicas = &size
		err = r.client.Update(context.TODO(), deployment)
		if err != nil {
			reqLogger.Error(err, "Failed to update Deployment.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return reconcile.Result{}, err
		}
	}

	// Check if the Service already exists, if not create a new one
	// NOTE: The Service is used to expose the Deployment.
	service := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pgbouncer.Name, Namespace: pgbouncer.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Service object
		ser := r.serviceForPgBouncer(pgbouncer)
		reqLogger.Info("Creating a new Service.", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
		err = r.client.Create(context.TODO(), ser)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Service.", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
			return reconcile.Result{}, err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service.")
		return reconcile.Result{}, err
	}

	// Update the PgBouncer status with the pod names
	// List the pods for this pgbouncer's deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(pgbouncer.Namespace),
		client.MatchingLabels(labelsForPgBouncer(pgbouncer.Name)),
	}
	err = r.client.List(context.TODO(), podList, listOpts...)
	if err != nil {
		reqLogger.Error(err, "Failed to list pods.", "PgBouncer.Namespace", pgbouncer.Namespace, "PgBouncer.Name", pgbouncer.Name)
		return reconcile.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, pgbouncer.Status.Nodes) {
		pgbouncer.Status.Nodes = podNames
		err := r.client.Status().Update(context.TODO(), pgbouncer)
		if err != nil {
			reqLogger.Error(err, "Failed to update PgBouncer status.")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// deploymentForPgBouncer returns a pgbouncer Deployment object
func (r *ReconcilePgBouncer) deploymentForPgBouncer(p *pgbounceroperatorv1alpha1.PgBouncer) *appsv1.Deployment {
	ls := labelsForPgBouncer(p.Name)
	replicas := p.Spec.Size

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.Name,
			Namespace: p.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:   "quay.io/pgbouncer-operator/pgbouncer",
						Name:    "pgbouncer",
						Command: []string{"sleep", "3600"},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 5432,
							Name:          "pgbouncer",
						}},
					}},
				},
			},
		},
	}
	// Set PgBouncer instance as the owner of the Deployment.
	controllerutil.SetControllerReference(p, dep, r.scheme)
	return dep
}

// serviceForPgBouncer function takes in a PgBouncer object and returns a Service for that object.
func (r *ReconcilePgBouncer) serviceForPgBouncer(p *pgbounceroperatorv1alpha1.PgBouncer) *corev1.Service {
	ls := labelsForPgBouncer(p.Name)
	ser := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.Name,
			Namespace: p.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{
				{
					Port: 5432,
					Name: p.Name,
				},
			},
		},
	}
	// Set PgBouncer instance as the owner of the Service.
	controllerutil.SetControllerReference(p, ser, r.scheme)
	return ser
}

// labelsForPgBouncer returns the labels for selecting the resources
// belonging to the given pgbouncer CR name.
func labelsForPgBouncer(name string) map[string]string {
	return map[string]string{"app": "pbgouncer", "pbgouncer_cr": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}
