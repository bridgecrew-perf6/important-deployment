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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	deploymentv1 "github.com/muralov/important-deployment/api/v1"
)

type DeploymentReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	UpdatedGeneration map[string]int64
	ReadyGeneration   map[string]int64
	CreatedGeneration map[string]int64
}

func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName(req.NamespacedName.String())
	log.Info("Reconciling " + req.NamespacedName.String())

	var deployment appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &deployment); err != nil {
		if apierrors.IsNotFound(err) {
			// c. when a deployment is DELETED
			// TODO: what to do with Notification CR is deleted
			// May be just send notifaction message. That's it!
			notificationErr := r.sendNotification("The deployment "+req.NamespacedName.String()+" is deleted.", ctx)
			delete(r.CreatedGeneration, deployment.ObjectMeta.Name)
			delete(r.UpdatedGeneration, deployment.ObjectMeta.Name)
			delete(r.ReadyGeneration, deployment.ObjectMeta.Name)
			// TODO: delete the Notification CR
			if notificationErr != nil {
				// retry sending notification
				return ctrl.Result{}, notificationErr
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Deployment")
		return ctrl.Result{}, err
	}

	// if deletionTimestamp is set, retry until it gets deleted fully
	if !deployment.DeletionTimestamp.IsZero() {
		return ctrl.Result{Requeue: true}, nil
	}

	err := r.createOrUpdateNotification(&deployment, ctx)
	if err != nil {
		// retry creating or sending notification if fails
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DeploymentReconciler) createOrUpdateNotification(deployment *appsv1.Deployment, ctx context.Context) error {
	log := log.FromContext(ctx)

	var notification deploymentv1.Notification
	if err := r.Get(ctx, types.NamespacedName{Namespace: deployment.Namespace, Name: deployment.Name}, &notification); err != nil {
		if apierrors.IsNotFound(err) {
			// a. when a deployment is freshly CREATED
			message := "The deployment " + deployment.Namespace + "/" + deployment.Name + " is created."
			notification = deploymentv1.Notification{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: deployment.Namespace,
					Name:      deployment.Name,
				},
				Spec: deploymentv1.NotificationSpec{
					Message:    message,
					Deployment: deployment,
				},
			}

			if r.CreatedGeneration[deployment.ObjectMeta.Name] != deployment.ObjectMeta.Generation {
				err = r.sendNotification(message, ctx)
				if err != nil {
					return err
				}
				r.CreatedGeneration[deployment.ObjectMeta.Name] = deployment.ObjectMeta.Generation
			}
			// create notification CR confirming notification was successfully sent
			err = r.Client.Create(ctx, &notification)
			if err != nil {
				return err
			}
			return nil
		}
		log.Error(err, "unable to fetch Deployment")
		return err
	}

	// create update notification only if spec is changed
	if deployment.ObjectMeta.Generation != notification.Spec.Deployment.ObjectMeta.Generation {
		message := "The deployment " + deployment.Namespace + "/" + deployment.Name + " is updated."
		if r.UpdatedGeneration[deployment.ObjectMeta.Name] != deployment.ObjectMeta.Generation {
			err := r.sendNotification(message, ctx)
			if err != nil {
				return err
			}
			r.UpdatedGeneration[deployment.ObjectMeta.Name] = deployment.ObjectMeta.Generation
		}

		diff :=  
		// create notification CR confirming notification was successfully sent
		notification.Spec = deploymentv1.NotificationSpec{
			Message:    message + "diff",
			Deployment: deployment,
		}
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return r.Client.Update(ctx, &notification)
		})
		if err != nil {
			return err
		}
	}

	// b. when a deployment is READY (all replicas up and running)
	if deployment.ObjectMeta.Generation == deployment.Status.ObservedGeneration &&
		*deployment.Spec.Replicas == deployment.Status.ReadyReplicas &&
		notification.Spec.ReadyGeneration != deployment.Generation {
		message := "The deployment " + deployment.Namespace + "/" + deployment.Name + " is ready."
		if r.ReadyGeneration[deployment.ObjectMeta.Name] != deployment.ObjectMeta.Generation {
			err := r.sendNotification(message, ctx)
			if err != nil {
				return err
			}
			r.ReadyGeneration[deployment.ObjectMeta.Name] = deployment.ObjectMeta.Generation
		}

		// create notification CR confirming notification was successfully sent
		notification.Spec = deploymentv1.NotificationSpec{
			Message:         message,
			Deployment:      deployment,
			ReadyGeneration: deployment.ObjectMeta.Generation,
		}
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return r.Client.Update(ctx, &notification)
		})
		if err != nil {
			return err
		}
		return nil
	}

	return nil
}

// sends a notification to an external service
func (r *DeploymentReconciler) sendNotification(message string, ctx context.Context) error {
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
		Owns(&deploymentv1.Notification{}).
		Complete(r)
}
