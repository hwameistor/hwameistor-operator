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

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/api/errors"

	log "github.com/sirupsen/logrus"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"github.com/hwameistor/hwameistor-operator/installhwamei"
	installrbac "github.com/hwameistor/hwameistor-operator/installhwamei/rbac"
	installldm "github.com/hwameistor/hwameistor-operator/installhwamei/localdiskmanager"
	installls "github.com/hwameistor/hwameistor-operator/installhwamei/localstorage"
	installscheduler "github.com/hwameistor/hwameistor-operator/installhwamei/scheduler"
	installadmissioncontroller "github.com/hwameistor/hwameistor-operator/installhwamei/admissioncontroller"
	installevictor "github.com/hwameistor/hwameistor-operator/installhwamei/evictor"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=hwameistor.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hwameistor.io,resources=clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hwameistor.io,resources=clusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Cluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Infof("Reconcile Cluster %s", req.Name)

	instance := &hwameistoriov1alpha1.Cluster{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Errorf("Get instance err: %v", err)
		return ctrl.Result{}, err
	}

	log.Infof("Cluster instance: %+v", instance)

	switch instance.Status.Phase {
	case hwameistoriov1alpha1.ClusterPhaseEmpty:
		instance.Status.Phase = hwameistoriov1alpha1.ClusterPhaseEnsuringTargetNamespaceExists
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			log.Errorf("Update status err: %v", err)
			return ctrl.Result{}, err
		}
	case hwameistoriov1alpha1.ClusterPhaseEnsuringTargetNamespaceExists:
		if err := installhwamei.EnsureTargetNamespaceExist(r.Client, instance.Spec.TargetNamespace); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}
		instance.Status.Phase = hwameistoriov1alpha1.ClusterPhaseToInstall
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			log.Errorf("Update status err: %v", err)
			return ctrl.Result{}, err
		}
	case hwameistoriov1alpha1.ClusterPhaseToInstall:
		if err := installhwamei.InstallCRDs(r.Client, instance.Spec.TargetNamespace); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}

		installrbac.SetRBAC(instance)
		if err := installrbac.InstallRBAC(r.Client); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}
		
		installldm.SetLDMDaemonSet(instance)
		if err := installldm.InstallLDM(r.Client); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}

		installldm.SetLDMCSIController(instance)
		if err := installldm.InstallLDMCSIController(r.Client); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}

		installls.SetLSDaemonSet(instance)
		if err := installls.InstallLS(r.Client); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}

		installls.SetLSCSIController(instance)
		if err := installls.InstallLSCSIController(r.Client); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}

		// installscheduler.SetSchedulerConfigMap(instance)
		if err := installscheduler.InstallSchedulerConfigMap(r.Client, instance.Spec.TargetNamespace); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}

		installscheduler.SetScheduler(instance)
		if err := installscheduler.InstallScheduler(r.Client); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}

		installadmissioncontroller.SetAdmissionController(instance)
		if err := installadmissioncontroller.InstallAdmissionController(r.Client); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}

		installadmissioncontroller.SetAdmissionControllerService(instance)
		if err := installadmissioncontroller.InstallAdmissionControllerService(r.Client); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}

		if err := installadmissioncontroller.InstallAdmissionControllerMutatingWebhookConfiguration(r.Client); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}

		installevictor.SetEvictor(instance)
		if err := installevictor.InstallEvictor(r.Client); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}

		instance.Status.Phase = hwameistoriov1alpha1.ClusterPhaseInstalled
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			log.Errorf("Update status err: %v", err)
		}
	default:
		log.Infof("Phase to do nothing: %v", instance.Status.Phase)
	}

	log.Infof("Instance phase: %v",instance.Status.Phase)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hwameistoriov1alpha1.Cluster{}).
		Complete(r)
}
