/*

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

package controllers

import (
	"context"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nginxappsv1 "example.com/napp-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
)

// NginxAppReconciler reconciles a NginxApp object
type NginxAppReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const (
	MAIN_PAGE = "index.html"
)

var (
	platformOwnerKey = ".metadata.controller"
	apiGVStr         = nginxappsv1.GroupVersion.String()
)

// +kubebuilder:rbac:groups=apps.example.com,resources=nginxapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.example.com,resources=nginxapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps/status,verbs=get

func (r *NginxAppReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("nginxapp", req.NamespacedName)

	// Fetch NginxApp object for further processing
	var app nginxappsv1.NginxApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if apierrs.IsNotFound(err) {
			// we'll ignore not-found errors, since they can't be fixed by an immediate requeue
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch NginxApp")
		return ctrl.Result{}, err
	}

	// Check if status was ever set
	shouldUpdateStatus := false
	if app.Status.Phase == "" {
		app.Status.Phase = nginxappsv1.PhasePending
		shouldUpdateStatus = true
	}

	updateStatus := func() {
		if shouldUpdateStatus {
			if err := r.Status().Update(ctx, &app); err != nil {
				if apierrs.IsConflict(err) {
					log.V(1).Info("unable to update NginxApp status (conflict), need to requeue reconcile")
				} else {
					log.Error(err, "unable to update NginxApp status")
				}
			}
		}
	}

	// Reconcile helper app
	helperPhase, err := r.ReconcileSimpleApp(ctx, req, &app, "helper", app.Spec.HelperApp)
	if err != nil {
		log.Error(err, "unable to reconcile helper app")
		updateStatus()
		return ctrl.Result{}, err
	}

	// If the helper app is not running, stop the reconcile process
	if helperPhase != nginxappsv1.PhaseRunning {
		log.Info("helper app not running yet, stopping reconcile")
		if helperPhase == nginxappsv1.PhaseFailed && app.Status.Phase != nginxappsv1.PhaseFailed {
			// Update phase to Failed if helper reconcile has failed
			app.Status.Phase = nginxappsv1.PhaseFailed
			shouldUpdateStatus = true
		}
		updateStatus()
		return ctrl.Result{}, nil
	}

	// Reconcile main app
	mainPhase, err := r.ReconcileSimpleApp(ctx, req, &app, "main", app.Spec.MainApp)
	if err != nil {
		log.Error(err, "unable to reconcile main app")
		updateStatus()
		return ctrl.Result{}, err
	}
	if app.Status.Phase != mainPhase {
		app.Status.Phase = mainPhase
		shouldUpdateStatus = true
	}

	updateStatus()
	return ctrl.Result{}, nil
}

func (r *NginxAppReconciler) ReconcileSimpleApp(ctx context.Context, req ctrl.Request, app *nginxappsv1.NginxApp, appSuffix string, simpleApp nginxappsv1.SimpleNginxApp) (phase nginxappsv1.ApplicationPhase, err error) {
	phase = nginxappsv1.PhasePending
	namespacedName := getNamespacedName(req.Name+"-"+appSuffix, req.Namespace)

	// Reconcile ConfigMap
	if err = r.ReconcileAppConfigMap(ctx, namespacedName, app, simpleApp); err != nil {
		phase = nginxappsv1.PhaseFailed
		return
	}

	// Reconcile Deployment
	var isRunning bool
	if isRunning, err = r.ReconcileAppDeployment(ctx, namespacedName, app, simpleApp); err != nil {
		phase = nginxappsv1.PhaseFailed
		return
	}

	// Reconcile Service
	if err = r.ReconcileAppService(ctx, namespacedName, app, simpleApp); err != nil {
		phase = nginxappsv1.PhaseFailed
		return
	}

	if isRunning {
		phase = nginxappsv1.PhaseRunning
	}

	return
}

func (r *NginxAppReconciler) ReconcileAppConfigMap(ctx context.Context, namespacedName types.NamespacedName, app *nginxappsv1.NginxApp, simpleApp nginxappsv1.SimpleNginxApp) (err error) {
	configMap := &corev1.ConfigMap{}
	if err = r.Get(ctx, namespacedName, configMap); err != nil {
		if apierrs.IsNotFound(err) {
			// not exists, need to create a new one
			configMap = &corev1.ConfigMap{
				ObjectMeta: v1.ObjectMeta{
					Name:      namespacedName.Name,
					Namespace: namespacedName.Namespace,
				},
				Data: map[string]string{
					MAIN_PAGE: simpleApp.WelcomeText,
				},
			}
			if err = ctrl.SetControllerReference(app, configMap, r.Scheme); err != nil {
				r.Log.Error(err, "unable to set controller reference for config map", "simpleApp", namespacedName)
				return
			}
			if err = r.Create(ctx, configMap); err != nil {
				r.Log.Error(err, "unable to create config map", "app", namespacedName)
			}
		} else {
			r.Log.Error(err, "unable to fetch config map", "app", namespacedName)
		}
	} else {
		configMap.Data[MAIN_PAGE] = simpleApp.WelcomeText
		if err = r.Update(ctx, configMap); err != nil {
			r.Log.Error(err, "unable to update config map", "app", namespacedName)
		}
	}

	return
}

func (r *NginxAppReconciler) ReconcileAppDeployment(ctx context.Context, namespacedName types.NamespacedName, app *nginxappsv1.NginxApp, simpleApp nginxappsv1.SimpleNginxApp) (isRunning bool, err error) {
	isRunning = false

	deployment := &appsv1.Deployment{}
	if err = r.Get(ctx, namespacedName, deployment); err != nil {
		if apierrs.IsNotFound(err) {
			// not exists, need to create a new one
			var replicas int32 = 1
			deployment = &appsv1.Deployment{
				ObjectMeta: v1.ObjectMeta{
					Name:      namespacedName.Name,
					Namespace: namespacedName.Namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &v1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name":     "nginxapp",
							"app.kubernetes.io/instance": app.Name,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: v1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name":     "nginxapp",
								"app.kubernetes.io/instance": app.Name,
							},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "app-files",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{Name: namespacedName.Name},
											//Items: []corev1.KeyToPath{
											//	{
											//		Key:  "name",
											//		Path: namespacedName.Name,
											//	},
											//},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: simpleApp.Image,
									Ports: []corev1.ContainerPort{
										{
											Name:          "http",
											ContainerPort: 8080,
											Protocol:      "TCP",
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "app-files",
											MountPath: "/app",
										},
									},
									ImagePullPolicy: "IfNotPresent",
								},
							},
						},
					},
				},
			}
			if err = ctrl.SetControllerReference(app, deployment, r.Scheme); err != nil {
				r.Log.Error(err, "unable to set controller reference for deployment", "simpleApp", namespacedName)
				return
			}
			if err = r.Create(ctx, deployment); err != nil {
				r.Log.Error(err, "unable to create deployment", "simpleApp", namespacedName)
			}
		} else {
			r.Log.Error(err, "unable to fetch deployment", "simpleApp", namespacedName)
		}
	} else {
		if deployment.Spec.Template.Spec.Containers[0].Image != simpleApp.Image {
			deployment.Spec.Template.Spec.Containers[0].Image = simpleApp.Image
			if err = r.Update(ctx, deployment); err != nil {
				r.Log.Error(err, "unable to update deployment", "simpleApp", namespacedName)
			}
		} else if deployment.Status.AvailableReplicas > 0 {
			isRunning = true
		}
	}

	return
}

func (r *NginxAppReconciler) ReconcileAppService(ctx context.Context, namespacedName types.NamespacedName, app *nginxappsv1.NginxApp, simpleApp nginxappsv1.SimpleNginxApp) (err error) {
	service := &corev1.Service{}
	if err = r.Get(ctx, namespacedName, service); err != nil {
		if apierrs.IsNotFound(err) {
			// not exists, need to create a new one
			service = &corev1.Service{
				ObjectMeta: v1.ObjectMeta{
					Name:      namespacedName.Name,
					Namespace: namespacedName.Namespace,
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Protocol:   "TCP",
							Port:       8080,
							TargetPort: intstr.IntOrString{IntVal: 8080},
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/name":     "nginxapp",
						"app.kubernetes.io/instance": app.Name,
					},
					Type: "ClusterIP",
				},
			}
			if err = ctrl.SetControllerReference(app, service, r.Scheme); err != nil {
				r.Log.Error(err, "unable to set controller reference for service", "simpleApp", namespacedName)
				return
			}
			if err = r.Create(ctx, service); err != nil {
				r.Log.Error(err, "unable to create service", "app", namespacedName)
			}
		} else {
			r.Log.Error(err, "unable to fetch service", "app", namespacedName)
		}
	}

	return
}

func getNamespacedName(name string, namespace string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
}

func getPlatformOwnerNames(owner *v1.OwnerReference) []string {
	if owner == nil {
		return nil
	}
	if owner.APIVersion != apiGVStr || owner.Kind != "InsurancePlatform" {
		return nil
	}
	return []string{owner.Name}
}

func (r *NginxAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create index for owned services
	if err := mgr.GetFieldIndexer().IndexField(&corev1.Service{}, platformOwnerKey, func(rawObj runtime.Object) []string {
		return getPlatformOwnerNames(v1.GetControllerOf(rawObj.(*corev1.Service)))
	}); err != nil {
		return err
	}

	// Create index for owned deployments
	if err := mgr.GetFieldIndexer().IndexField(&appsv1.Deployment{}, platformOwnerKey, func(rawObj runtime.Object) []string {
		return getPlatformOwnerNames(v1.GetControllerOf(rawObj.(*appsv1.Deployment)))
	}); err != nil {
		return err
	}

	// Create index for owned config maps
	if err := mgr.GetFieldIndexer().IndexField(&corev1.ConfigMap{}, platformOwnerKey, func(rawObj runtime.Object) []string {
		return getPlatformOwnerNames(v1.GetControllerOf(rawObj.(*corev1.ConfigMap)))
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&nginxappsv1.NginxApp{}).
		Watches(
			&source.Kind{Type: &appsv1.Deployment{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &nginxappsv1.NginxApp{},
			}).
		Complete(r)
}
