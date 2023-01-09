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
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"github.com/hwameistor/hwameistor-operator/installhwamei"
	installadmissioncontroller "github.com/hwameistor/hwameistor-operator/installhwamei/admissioncontroller"
	installapiserver "github.com/hwameistor/hwameistor-operator/installhwamei/apiserver"
	installmetrics "github.com/hwameistor/hwameistor-operator/installhwamei/metrics"
	installdrbd "github.com/hwameistor/hwameistor-operator/installhwamei/drbd"
	installevictor "github.com/hwameistor/hwameistor-operator/installhwamei/evictor"
	installldm "github.com/hwameistor/hwameistor-operator/installhwamei/localdiskmanager"
	installls "github.com/hwameistor/hwameistor-operator/installhwamei/localstorage"
	installrbac "github.com/hwameistor/hwameistor-operator/installhwamei/rbac"
	installscheduler "github.com/hwameistor/hwameistor-operator/installhwamei/scheduler"
	installstorageclass "github.com/hwameistor/hwameistor-operator/installhwamei/storageclass"
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

	newInstance := instance.DeepCopy()

	reReconcile, err := installhwamei.EnsureTargetNamespaceExist(r.Client, newInstance.Spec.TargetNamespace)
	if err != nil {
		log.Errorf("Install err: %v", err)
		return ctrl.Result{}, err
	}
	if reReconcile {
		return ctrl.Result{Requeue: true}, nil
	}

	if !newInstance.Status.InstalledCRDS {
		if err := installhwamei.InstallCRDs(r.Client, newInstance.Spec.TargetNamespace); err != nil {
			log.Errorf("Install err: %v", err)
			return ctrl.Result{}, err
		}
		newInstance.Status.InstalledCRDS = true
		if err := r.Client.Status().Update(ctx, newInstance); err != nil {
			log.Errorf("Update status err: %v", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	rbacMaintainer := installrbac.NewRBACMaintainer(r.Client, newInstance)
	if err := rbacMaintainer.Ensure(); err != nil {
		log.Errorf("Ensure RBAC err: %v", err)
		return ctrl.Result{}, err
	}

	ldmDaemonSetMaintainer := installldm.NewLocalDiskManagerMaintainer(r.Client, newInstance)
	newInstance, err = ldmDaemonSetMaintainer.Ensure()
	if err != nil {
		log.Errorf("Ensure LocalDiskManager DaemonSet err: %v", err)
		return ctrl.Result{}, err
	}

	ldmCSIMaintainer := installldm.NewLDMCSIMaintainer(r.Client, newInstance)
	newInstance, err = ldmCSIMaintainer.Ensure()
	if err != nil {
		log.Errorf("Ensure LDM CSIController err: %v", err)
		return ctrl.Result{}, err
	}

	if ldm := newInstance.Status.LocalDiskManager; ldm != nil {
		instances := ldm.Instances
		csi := ldm.CSI
		if (instances != nil) && (csi != nil) {
			if (instances.AvailablePodCount == instances.DesiredPodCount) && (csi.AvailablePodCount == csi.DesiredPodCount) {
				newInstance.Status.LocalDiskManager.Health = "Normal"
			} else {
				newInstance.Status.LocalDiskManager.Health = "Abnormal"
			}
		}
	}

	lsDaemonSetMaintainer := installls.NewLocalStorageMaintainer(r.Client, newInstance)
	newInstance, err = lsDaemonSetMaintainer.Ensure()
	if err != nil {
		log.Errorf("Ensure LocalStorage DaemonSet err: %v", err)
		return ctrl.Result{}, err
	}

	lsCSIMaintainer := installls.NewLSCSIMaintainer(r.Client, newInstance)
	newInstance, err = lsCSIMaintainer.Ensure()
	if err != nil {
		log.Errorf("Ensure LS CSIController err: %v", err)
		return ctrl.Result{}, err
	}

	if ls := newInstance.Status.LocalStorage; ls != nil {
		instances := ls.Instances
		csi := ls.CSI
		if (instances != nil) && (csi != nil) {
			if (instances.AvailablePodCount == instances.DesiredPodCount) && (csi.AvailablePodCount == csi.DesiredPodCount) {
				newInstance.Status.LocalStorage.Health = "Normal"
			} else {
				newInstance.Status.LocalStorage.Health = "Abnormal"
			}
		}
	}

	admissionControllerMaintainer := installadmissioncontroller.NewAdmissionControllerMaintainer(r.Client, newInstance)
	newInstance, err = admissionControllerMaintainer.Ensure()
	if err != nil {
		log.Errorf("Ensure AdmissionController err: %v", err)
		return ctrl.Result{}, err
	}

	if admissionController := newInstance.Status.AdmissionController; admissionController != nil {
		instances := admissionController.Instances
		if instances != nil {
			if instances.AvailablePodCount == instances.DesiredPodCount {
				newInstance.Status.AdmissionController.Health = "Normal"
			} else {
				newInstance.Status.AdmissionController.Health = "Abnormal"
			}
		}
	}

	admissionControllerServiceMaintainer := installadmissioncontroller.NewAdmissionControllerServiceMaintainer(r.Client, newInstance)
	if err := admissionControllerServiceMaintainer.Ensure(); err != nil {
		log.Errorf("Ensure AdmissionController Service err: %v", err)
		return ctrl.Result{}, err
	}

	schedulerConfigMapMaintainer := installscheduler.NewSchedulerConfigMapMaintainer(r.Client, newInstance)
	if err := schedulerConfigMapMaintainer.Ensure(); err != nil {
		log.Errorf("Ensure Scheduler ConfigMap err: %v", err)
		return ctrl.Result{}, err
	}

	schedulerMaintainer := installscheduler.NewSchedulerMaintainer(r.Client, newInstance)
	newInstance, err = schedulerMaintainer.Ensure()
	if err != nil {
		log.Errorf("Ensure Scheduler err: %v", err)
		return ctrl.Result{}, err
	}

	if scheduler := newInstance.Status.Scheduler; scheduler != nil {
		instances := scheduler.Instances
		if instances != nil {
			if instances.AvailablePodCount == instances.DesiredPodCount {
				newInstance.Status.Scheduler.Health = "Normal"
			} else {
				newInstance.Status.Scheduler.Health = "Abnormal"
			}
		}
	}

	evictorMaintainer := installevictor.NewEvictorMaintainer(r.Client, newInstance)
	newInstance, err = evictorMaintainer.Ensure()
	if err != nil {
		log.Errorf("Ensure Evictor err: %v", err)
		return ctrl.Result{}, err
	}

	if evictor := newInstance.Status.Evictor; evictor != nil {
		instances := evictor.Instances
		if instances != nil {
			if instances.AvailablePodCount == instances.DesiredPodCount {
				newInstance.Status.Evictor.Health = "Normal"
			} else {
				newInstance.Status.Evictor.Health = "Abnormal"
			}
		}
	}

	apiServerMaintainer := installapiserver.NewApiServerMaintainer(r.Client, newInstance)
	newInstance, err = apiServerMaintainer.Ensure()
	if err != nil {
		log.Errorf("Ensure ApiServer err: %v", err)
		return ctrl.Result{}, err
	}

	if apiServer := newInstance.Status.ApiServer; apiServer != nil {
		instances := apiServer.Instances
		if instances != nil {
			if instances.AvailablePodCount == instances.DesiredPodCount {
				newInstance.Status.ApiServer.Health = "Normal"
			} else {
				newInstance.Status.ApiServer.Health = "Abnormal"
			}
		}
	}

	apiServerServiceMaintainer := installapiserver.NewApiServerServiceMaintainer(r.Client, newInstance)
	if err := apiServerServiceMaintainer.Ensure(); err != nil {
		log.Errorf("Ensure ApiServer Service err: %v", err)
		return ctrl.Result{}, err
	}

	metricsMaintainer := installmetrics.NewMetricsMaintainer(r.Client, newInstance)
	newInstance, err = metricsMaintainer.Ensure()
	if err != nil {
		log.Errorf("Ensure Metrics Collector err: %v", err)
		return ctrl.Result{}, err
	}

	if metrics := newInstance.Status.Metrics; metrics != nil {
		instances := metrics.Instances
		if instances != nil {
			if instances.AvailablePodCount == instances.DesiredPodCount {
				newInstance.Status.Metrics.Health = "Normal"
			} else {
				newInstance.Status.Metrics.Health = "Abnormal"
			}
		}
	}

	metricsServiceMaintainer := installmetrics.NewMetricsServiceMaintainer(r.Client, newInstance)
	if err := metricsServiceMaintainer.Ensure(); err != nil {
		log.Errorf("Ensure Metrics Service err: %v", err)
		return ctrl.Result{}, err
	}

	if instance.Spec.StorageClass.Enable {
		storageClassMaintainer := installstorageclass.NewStorageClassMaintainer(r.Client, newInstance)
		if err := storageClassMaintainer.Ensure(); err != nil {
			log.Errorf("Ensure StorageClass err: %v", err)
			return ctrl.Result{}, err
		}
	}

	if instance.Spec.DRBD.Enable {
		installdrbd.HandelDRBDConfigs(instance)
		if err := installdrbd.CreateDRBDAdapter(r.Client); err != nil {
			log.Errorf("Create DRBD Adapter err: %v", err)
			return ctrl.Result{}, err
		}
	}

	if reflect.DeepEqual(instance, newInstance) {
		log.Infof("No need to update status")
	} else {
		if err := r.Client.Status().Update(ctx, newInstance); err != nil {
			log.Errorf("Update status err: %v", err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hwameistoriov1alpha1.Cluster{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
