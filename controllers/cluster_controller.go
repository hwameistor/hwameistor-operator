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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"

	hwameistoroperatorv1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"github.com/hwameistor/hwameistor-operator/pkg/install"
	"github.com/hwameistor/hwameistor-operator/pkg/install/admissioncontroller"
	"github.com/hwameistor/hwameistor-operator/pkg/install/apiserver"
	"github.com/hwameistor/hwameistor-operator/pkg/install/auditor"
	"github.com/hwameistor/hwameistor-operator/pkg/install/drbd"
	"github.com/hwameistor/hwameistor-operator/pkg/install/evictor"
	"github.com/hwameistor/hwameistor-operator/pkg/install/exporter"
	"github.com/hwameistor/hwameistor-operator/pkg/install/failoverassistant"
	"github.com/hwameistor/hwameistor-operator/pkg/install/ldmcsicontroller"
	"github.com/hwameistor/hwameistor-operator/pkg/install/localdiskmanager"
	"github.com/hwameistor/hwameistor-operator/pkg/install/localstorage"
	"github.com/hwameistor/hwameistor-operator/pkg/install/lscsicontroller"
	"github.com/hwameistor/hwameistor-operator/pkg/install/pvcautoresizer"
	"github.com/hwameistor/hwameistor-operator/pkg/install/localdiskactioncontroller"
	"github.com/hwameistor/hwameistor-operator/pkg/install/rbac"
	"github.com/hwameistor/hwameistor-operator/pkg/install/scheduler"
	"github.com/hwameistor/hwameistor-operator/pkg/install/storageclass"
	"github.com/hwameistor/hwameistor-operator/pkg/install/ui"
	"github.com/hwameistor/hwameistor-operator/pkg/install/utils"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	ClusterSpecGeneration int64
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

	instance := &hwameistoroperatorv1alpha1.Cluster{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Errorf("Get instance err: %v", err)
		return ctrl.Result{}, err
	}

	fulfilledClusterInstance := FulfillClusterInstance(instance)
	if !reflect.DeepEqual(instance.Spec, fulfilledClusterInstance.Spec) {
		log.Infof("Not equal between origin instance and fulfilled instance")
		log.Infof("Instance spec: %+v", instance.Spec)
		log.Infof("FulfilledClusterInstance spec: %+v", fulfilledClusterInstance.Spec)
		if err := r.Client.Update(ctx, fulfilledClusterInstance); err != nil {
			log.Errorf("Update Cluster err: %v", err)
			return ctrl.Result{}, err
		} else {
			log.Infof("Updated Cluster successfully")
			return ctrl.Result{}, nil
		}
	}

	newInstance := fulfilledClusterInstance.DeepCopy()

	reReconcile, err := install.EnsureTargetNamespaceExist(r.Client, newInstance.Spec.TargetNamespace)
	if err != nil {
		log.Errorf("Install err: %v", err)
		return ctrl.Result{}, err
	}
	if reReconcile {
		return ctrl.Result{Requeue: true}, nil
	}
	log.Infof("Target namespace check passed")

	// status.installedCRDs true bool value will cause hwameistor crds not updated when upgrade,
	// so we turn status.installedCRDS to false bool value here once spec generation changed.
	// That will ensure hwameistor crds updating not missed when upgrading.
	if r.ClusterSpecGeneration != newInstance.Generation {
		log.Infof("cached cluster spec generation:%v, gotten cluster generation: %v", r.ClusterSpecGeneration, newInstance.Generation)
		log.Infof("going to set status.installedCRDS to false bool value")
		newInstance.Status.InstalledCRDS = false
		if err := r.Client.Status().Update(ctx, newInstance); err != nil {
			log.Errorf("Update status err: %v", err)
			return ctrl.Result{}, err
		}
		r.ClusterSpecGeneration = newInstance.Generation
		return ctrl.Result{}, nil
	}

	if !newInstance.Status.InstalledCRDS {
		if err := install.InstallCRDs(r.Client, newInstance.Spec.TargetNamespace); err != nil {
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
	log.Infof("Handling of crds passed")

	if err := rbac.NewMaintainer(r.Client, newInstance).Ensure(); err != nil {
		log.Errorf("Ensure RBAC err: %v", err)
		return ctrl.Result{}, err
	}
	log.Infof("RBAC Ensured")

	newInstance, err = localdiskmanager.NewMaintainer(r.Client, newInstance).Ensure()
	if err != nil {
		log.Errorf("Ensure LocalDiskManager DaemonSet err: %v", err)
		return ctrl.Result{}, err
	}

	newInstance, err = ldmcsicontroller.NewMaintainer(r.Client, newInstance).Ensure()
	if err != nil {
		log.Errorf("Ensure LDM CSIController err: %v", err)
		return ctrl.Result{}, err
	}

	if ldm := newInstance.Status.ComponentStatus.LocalDiskManager; ldm != nil {
		instances := ldm.Instances
		csi := ldm.CSI
		if (instances != nil) && (csi != nil) {
			if (instances.AvailablePodCount == instances.DesiredPodCount) && (csi.AvailablePodCount == csi.DesiredPodCount) {
				newInstance.Status.ComponentStatus.LocalDiskManager.Health = "Normal"
			} else {
				newInstance.Status.ComponentStatus.LocalDiskManager.Health = "Abnormal"
			}
		}
	}

	if !newInstance.Spec.LocalStorage.Disable {
		newInstance, err = localstorage.NewMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("Ensure LocalStorage DaemonSet err: %v", err)
			return ctrl.Result{}, err
		}

		newInstance, err = lscsicontroller.NewMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("Ensure LS CSIController err: %v", err)
			return ctrl.Result{}, err
		}

		if ls := newInstance.Status.ComponentStatus.LocalStorage; ls != nil {
			instances := ls.Instances
			csi := ls.CSI
			if (instances != nil) && (csi != nil) {
				if (instances.AvailablePodCount == instances.DesiredPodCount) && (csi.AvailablePodCount == csi.DesiredPodCount) {
					newInstance.Status.ComponentStatus.LocalStorage.Health = "Normal"
				} else {
					newInstance.Status.ComponentStatus.LocalStorage.Health = "Abnormal"
				}
			}
		}
	}

	if !newInstance.Spec.AdmissionController.Disable {
		newInstance, err = admissioncontroller.NewAdmissionControllerMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("Ensure AdmissionController err: %v", err)
			return ctrl.Result{}, err
		}

		if admissionController := newInstance.Status.ComponentStatus.AdmissionController; admissionController != nil {
			instances := admissionController.Instances
			if instances != nil {
				if instances.AvailablePodCount == instances.DesiredPodCount {
					newInstance.Status.ComponentStatus.AdmissionController.Health = "Normal"
				} else {
					newInstance.Status.ComponentStatus.AdmissionController.Health = "Abnormal"
				}
			}
		}

		if err := admissioncontroller.NewAdmissionControllerServiceMaintainer(r.Client, newInstance).Ensure(); err != nil {
			log.Errorf("Ensure AdmissionController Service err: %v", err)
			return ctrl.Result{}, err
		}
	}

	if !newInstance.Spec.Scheduler.Disable {
		if err := scheduler.NewSchedulerConfigMapMaintainer(r.Client, newInstance).Ensure(); err != nil {
			log.Errorf("Ensure Scheduler ConfigMap err: %v", err)
			return ctrl.Result{}, err
		}

		newInstance, err = scheduler.NewSchedulerMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("Ensure Scheduler err: %v", err)
			return ctrl.Result{}, err
		}

		if scheduler := newInstance.Status.ComponentStatus.Scheduler; scheduler != nil {
			instances := scheduler.Instances
			if instances != nil {
				if instances.AvailablePodCount == instances.DesiredPodCount {
					newInstance.Status.ComponentStatus.Scheduler.Health = "Normal"
				} else {
					newInstance.Status.ComponentStatus.Scheduler.Health = "Abnormal"
				}
			}
		}
	}

	if !newInstance.Spec.Evictor.Disable {
		newInstance, err = evictor.NewMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("Ensure Evictor err: %v", err)
			return ctrl.Result{}, err
		}

		if evictor := newInstance.Status.ComponentStatus.Evictor; evictor != nil {
			instances := evictor.Instances
			if instances != nil {
				if instances.AvailablePodCount == instances.DesiredPodCount {
					newInstance.Status.ComponentStatus.Evictor.Health = "Normal"
				} else {
					newInstance.Status.ComponentStatus.Evictor.Health = "Abnormal"
				}
			}
		}
	}

	if !newInstance.Spec.Auditor.Disable {
		newInstance, err = auditor.NewAuditorMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("ensure auditor err: %v", err)
			return ctrl.Result{}, err
		}

		if auditor := newInstance.Status.ComponentStatus.Auditor; auditor != nil {
			instances := auditor.Instances
			if instances != nil {
				if instances.AvailablePodCount == instances.DesiredPodCount {
					newInstance.Status.ComponentStatus.Auditor.Health = "Normal"
				} else {
					newInstance.Status.ComponentStatus.Auditor.Health = "Abnormal"
				}
			}
		}
	}

	if !newInstance.Spec.FailoverAssistant.Disable {
		newInstance, err = failoverassistant.NewFailoverAssistantMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("ensure failover-assistant err: %v", err)
			return ctrl.Result{}, err
		}

		if assistant := newInstance.Status.ComponentStatus.FailoverAssistant; assistant != nil {
			instances := assistant.Instances
			if instances != nil {
				if instances.AvailablePodCount == instances.DesiredPodCount {
					newInstance.Status.ComponentStatus.FailoverAssistant.Health = "Normal"
				} else {
					newInstance.Status.ComponentStatus.FailoverAssistant.Health = "Abnormal"
				}
			}
		}
	}

	if !newInstance.Spec.PVCAutoResizer.Disable {
		newInstance, err = pvcautoresizer.NewPVCAutoResizerMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("ensure pvc-autoresizer err: %v", err)
			return ctrl.Result{}, err
		}

		if autoresizer := newInstance.Status.ComponentStatus.PVCAutoResizer; autoresizer != nil {
			instances := autoresizer.Instances
			if instances != nil {
				if instances.AvailablePodCount == instances.DesiredPodCount {
					newInstance.Status.ComponentStatus.PVCAutoResizer.Health = "Normal"
				} else {
					newInstance.Status.ComponentStatus.PVCAutoResizer.Health = "Abnormal"
				}
			}
		}
	}

	if !newInstance.Spec.LocalDiskActionController.Disable {
		newInstance, err = localdiskactioncontroller.NewActionControllerMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("ensure localdiskactioncontroller err: %v", err)
			return ctrl.Result{}, err
		}

		if controller := newInstance.Status.ComponentStatus.LocalDiskActionController; controller != nil {
			instances := controller.Instances
			if instances != nil {
				if instances.AvailablePodCount == instances.DesiredPodCount {
					newInstance.Status.ComponentStatus.LocalDiskActionController.Health = "Normal"
				} else {
					newInstance.Status.ComponentStatus.LocalDiskActionController.Health = "Abnormal"
				}
			}
		}
	}

	if !newInstance.Spec.ApiServer.Disable {
		newInstance, err = apiserver.NewApiServerMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("Ensure ApiServer err: %v", err)
			return ctrl.Result{}, err
		}

		if apiServer := newInstance.Status.ComponentStatus.ApiServer; apiServer != nil {
			instances := apiServer.Instances
			if instances != nil {
				if instances.AvailablePodCount == instances.DesiredPodCount {
					newInstance.Status.ComponentStatus.ApiServer.Health = "Normal"
				} else {
					newInstance.Status.ComponentStatus.ApiServer.Health = "Abnormal"
				}
			}
		}

		if err := apiserver.NewApiServerServiceMaintainer(r.Client, newInstance).Ensure(); err != nil {
			log.Errorf("Ensure ApiServer Service err: %v", err)
			return ctrl.Result{}, err
		}
	}

	if !newInstance.Spec.Exporter.Disable {
		newInstance, err = exporter.NewExporterMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("Ensure Exporter Collector err: %v", err)
			return ctrl.Result{}, err
		}

		if exporter := newInstance.Status.ComponentStatus.Exporter; exporter != nil {
			instances := exporter.Instances
			if instances != nil {
				if instances.AvailablePodCount == instances.DesiredPodCount {
					newInstance.Status.ComponentStatus.Exporter.Health = "Normal"
				} else {
					newInstance.Status.ComponentStatus.Exporter.Health = "Abnormal"
				}
			}
		}

		if err := exporter.NewExporterServiceMaintainer(r.Client, newInstance).Ensure(); err != nil {
			log.Errorf("Ensure Exporter Service err: %v", err)
			return ctrl.Result{}, err
		}
	}

	if !newInstance.Spec.UI.Disable {
		newInstance, err = ui.NewUIMaintainer(r.Client, newInstance).Ensure()
		if err != nil {
			log.Errorf("Ensure UI err: %v", err)
			return ctrl.Result{}, err
		}

		if err := ui.NewUIServiceMaintainer(r.Client, newInstance).Ensure(); err != nil {
			log.Errorf("Ensure UI Service err: %v", err)
			return ctrl.Result{}, err
		}
	}

	if !newInstance.Spec.DRBD.Disable {
		if !newInstance.Status.DRBDAdapterCreated {
			drbd.HandelDRBDConfigs(instance)
			drbdAdapterJobCreatedNum, err := drbd.CreateDRBDAdapter(r.Client)
			if err != nil {
				log.Errorf("Create DRBD Adapter err: %v", err)
				return ctrl.Result{}, err
			} else {
				newInstance.Status.DRBDAdapterCreated = true
				newInstance.Status.DRBDAdapterCreatedJobNum = drbdAdapterJobCreatedNum
			}
		}
	}

	// Use Phase to adopt DiskReserveState, notice that don't lose the value of DiskReserveState
	if newInstance.Status.DiskReserveState != "" && newInstance.Status.Phase == "" {
		newInstance.Status.Phase = newInstance.Status.DiskReserveState
	}

	switch newInstance.Status.Phase {
	case "":
		if utils.CheckComponentsInstalledSuccessfully(r.Client, newInstance) {
			newInstance.Status.Phase = "ToReserve"
		}
	case "ToReserve":
		log.Infof("sleep 2 minutes to wait for localdiskmanager created localdisks")
		time.Sleep(time.Minute*2)
		log.Infof("2 minutes waited, going to handle localdisks")
		if err := utils.ReserveDisk(newInstance, r.Client); err != nil {
			log.Errorf("Reserve Disk err: %v", err)
			return ctrl.Result{}, err
		}
		newInstance.Status.Phase = "Reserved"
	case "Reserved":
		log.Infof("Disk Reserved")
		if newInstance.Spec.NotClaimDisk {
			newInstance.Status.Phase = "CreatedLDC"
			log.Infof("Not ClaimDisk")
			break
		}
		localDisks, err := utils.ListLocalDisks(r.Client)
		if err != nil {
			log.Errorf("List Disks err: %v", err)
			return ctrl.Result{}, err
		}
		log.Infof("LocalDisks: %+v", localDisks)
		localDisks = utils.SiftAvailableAndUnreservedDisks(localDisks)
		log.Infof("Sifted LocalDisks: %+v", localDisks)
		localDiskClaims := utils.GenerateLocalDiskClaimsToCreateAccordingToLocalDisks(localDisks)
		log.Infof("LocalDiskClaims to create: %+v", localDiskClaims)
		if err := utils.CreateLocalDiskClaims(r.Client, localDiskClaims); err != nil {
			log.Errorf("Create LocalDiskClaims err: %v", err)
			return ctrl.Result{}, err
		}
		newInstance.Status.Phase = "CreatedLDC"
	case "CreatedLDC":
		if !newInstance.Spec.StorageClass.Disable {
			// if err := storageclass.NewMaintainer(r.Client, newInstance).Ensure(); err != nil {
			// 	log.Errorf("Ensure StorageClass err: %v", err)
			// 	return ctrl.Result{}, err
			// }
			storageclass.EnsureWatcherStarted(r.Client, req.NamespacedName)
			log.Infof("LocalStorageNodeWatcher started")
			storageclass.EnsureLDNWatcherStarted(r.Client, req.NamespacedName)
			log.Infof("LocalDiskNodeWatcher started")
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
		For(&hwameistoroperatorv1alpha1.Cluster{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

func FulfillClusterInstance(clusterInstance *hwameistoroperatorv1alpha1.Cluster) *hwameistoroperatorv1alpha1.Cluster {
	newClusterInstance := clusterInstance.DeepCopy()

	newClusterInstance = install.FulfillTargetNamespaceSpec(newClusterInstance)
	newClusterInstance = rbac.FulfillRBACSpec(newClusterInstance)
	newClusterInstance = localdiskmanager.FulfillLDMDaemonsetSpec(newClusterInstance)
	newClusterInstance = ldmcsicontroller.FulfillLDMCSISpec(newClusterInstance)
	newClusterInstance = localstorage.FulfillLSDaemonsetSpec(newClusterInstance)
	newClusterInstance = lscsicontroller.FulfillLSCSISpec(newClusterInstance)
	newClusterInstance = admissioncontroller.FulfillAdmissionControllerSpec(newClusterInstance)
	newClusterInstance = scheduler.FulfillSchedulerSpec(newClusterInstance)
	newClusterInstance = evictor.FulfillEvictorSpec(newClusterInstance)
	newClusterInstance = apiserver.FulfillApiServerSpec(newClusterInstance)
	newClusterInstance = exporter.FulfillExporterSpec(newClusterInstance)
	newClusterInstance = ui.FulfillUISpec(newClusterInstance)
	newClusterInstance = storageclass.FulfillStorageClassSpec(newClusterInstance)
	newClusterInstance = drbd.FulfillDRBDSpec(newClusterInstance)

	return newClusterInstance
}
