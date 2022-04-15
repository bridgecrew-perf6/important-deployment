package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type DeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling " + req.NamespacedName.String())

	var deployment appsv1.Deployment

	if err := r.Get(ctx, req.NamespacedName, &deployment); err != nil {
		if apierrors.IsNotFound(err) {
			// c. when a deployment is DELETED
			notificationErr := r.createNotification("The deployment "+req.NamespacedName.String()+" is deleted.", ctx)
			if notificationErr != nil {
				// retry sending notification
				return ctrl.Result{}, notificationErr
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Deployment")
		return ctrl.Result{}, err
	}

	// A notification about the name of the changed deployment-resource + the changes which were made.

	// There should be 3 types of notification
	// a. when a deployment is freshly CREATED

	if deployment.ObjectMeta.Generation == 1 {

	}

	// b. when a deployment is READY (all replicas up and running)
	// TODO: don't s end notification if operator restarts
	if *deployment.Spec.Replicas == deployment.Status.ReadyReplicas {
		r.createNotification("The deployment "+req.NamespacedName.String()+" is ready.", ctx)
	}

	// fmt.Println("The changed deployment: " + deployment.Namespace + " / " + deployment.Name)

	return ctrl.Result{}, nil
}

func (r *DeploymentReconciler) createNotification(message string, ctx context.Context) error {
	log := log.FromContext(ctx)
	notificationBody, _ := json.Marshal(map[string]string{
		"message":        message,
		"deploymentname": "devops/nginx-deployment",
	})
	requestBody := bytes.NewBuffer(notificationBody)
	resp, err := http.Post("https://httpbin.org/post", "application/json", requestBody)
	// TODO: check the http status too
	if err != nil {
		return err
	}

	fmt.Println("Got the following response: ")
	b, err := io.ReadAll(resp.Body)
	// b, err := ioutil.ReadAll(resp.Body)  Go.1.15 and earlier
	if err != nil {
		log.Error(err, "cannot convert the response body to string")
	}
	fmt.Println(string(b))

	return nil
}

type Notification struct {
	ID             string
	DeplyomentName string
	Message        string
}

// func (r *DeploymentReconciler) getNotifications(namespacedName string, ctx context.Context) ([]Notification, error) {
// 	log := log.FromContext(ctx)
// 	resp, err := http.Get("https://httpbin.org/anything")
// 	// TODO: check the http status too
// 	if err != nil {
// 		return err
// 	}

// 	fmt.Println("Got the following response: ")
// 	b, err := io.ReadAll(resp.Body)
// 	// b, err := ioutil.ReadAll(resp.Body)  Go.1.15 and earlier
// 	if err != nil {
// 		log.Error(err, "cannot convert the response body to string")
// 	}
// 	fmt.Println(string(b))

// 	return nil
// }

// func FilterChangesForNamespace() predicate.Predicate {
// 	return predicate.Funcs{
// 		CreateFunc: func(e event.CreateEvent) bool {
// 			return true
// 		},
// 		UpdateFunc: func(e event.UpdateEvent) bool {
// 			deepEqual := reflect.DeepEqual(e.ObjectOld.DeepCopyObject(), e.ObjectNew.DeepCopyObject())
// 			return !deepEqual
// 		},
// 		DeleteFunc: func(e event.DeleteEvent) bool {
// 			return true
// 		},
// 	}
// }

func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	isSomeCiSystem, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{"importantDeployment": "some-ci-system"},
	})
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		WithEventFilter(isSomeCiSystem).
		Complete(r)
}
