package drbd

import (
	"context"
	"fmt"
	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	operatorv1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"github.com/hwameistor/hwameistor-operator/pkg/install"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

var defaultDeployOnMaster = "no"
var defaultImageRegistry = "ghcr.io"
var defaultShipperRepository = "hwameistor/drbd9-shipper"
var defaultImagePullPolicy = "IfNotPresent"
var defaultDRBDVersion = "v9.0.32-1"
var defaultShipperChar = "v0.4.2"
var defaultDRBDUpgrade = "no"
var defaultCheckHostName = "no"
var defaultUseAffinity = "no"
var defaultNodeSelectTerms = []corev1.NodeSelectorTerm{
	{
		MatchExpressions: []corev1.NodeSelectorRequirement{
			{
				Key:      "node-role.kubernetes.io/master",
				Operator: corev1.NodeSelectorOpDoesNotExist,
			},
			{
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: corev1.NodeSelectorOpDoesNotExist,
			},
		},
	},
}
var defaultChartVersion = "v0.4.2"

var distroRegexMap = map[string]string{
	"(red hat enterprise|centos|almalinux|rocky linux) .*?7(\\.|\\s|$)": "rhel7",
	"(red hat enterprise|centos|almalinux|rocky linux) .*?8(\\.|\\s|$)": "rhel8",
	"(red hat enterprise|centos|almalinux|rocky linux) .*?9(\\.|\\s|$)": "rhel9",
	"ubuntu .*?18(\\.|\\s|$)": "bionic",
	"ubuntu .*?20(\\.|\\s|$)": "focal",
	"ubuntu .*?22(\\.|\\s|$)": "jammy",
	"kylin .*?v10(\\.|\\s|$)": "kylin10",
}

var backoffLimit0 = int32(0)
var terminationGracePeriodSeconds0 = int64(0)

var deployOnMaster = false
var drbdVersion string
var tag string
var shipperChar string
var imagePullPolicy string
var shapperImageRegistry string
var shapperImageRepository string

// var imageRepoOwner string
var distroImageRepository string
var chartVersion string
var upgrade string
var checkHostName string
var useAffinity string
var nodeAffinity corev1.NodeAffinity
var namespace string

func HandelDRBDConfigs(clusterInstance *hwameistoriov1alpha1.Cluster) {
	drbdConfigs := clusterInstance.Spec.DRBD
	if drbdConfigs == nil {
		return
	}

	if drbdConfigs.DeployOnMaster == "yes" {
		deployOnMaster = true
	}

	namespace = clusterInstance.Spec.TargetNamespace

	drbdVersion = drbdConfigs.DRBDVersion
	tag = drbdVersion
	shapperImageRegistry = drbdConfigs.Shipper.Registry
	shapperImageRepository = drbdConfigs.Shipper.Repository
	shipperChar = drbdConfigs.Shipper.Tag
	imagePullPolicy = drbdConfigs.ImagePullPolicy
	chartVersion = drbdConfigs.ChartVersion
	upgrade = drbdConfigs.Upgrade
	checkHostName = drbdConfigs.CheckHostName
	useAffinity = drbdConfigs.UseAffinity
	nodeAffinity = *drbdConfigs.NodeAffinity
}

func CreateDRBDAdapter(instance *hwameistoriov1alpha1.Cluster, cli client.Client) error {
	nodeList := corev1.NodeList{}
	if err := cli.List(context.TODO(), &nodeList); err != nil {
		log.Errorf("List nodes err: %v", err)
		return err
	}
	ownerReference := []v1.OwnerReference{*v1.NewControllerRef(instance, schema.FromAPIVersionAndKind("hwameistor.io/v1alpha1", "Cluster"))}
	for _, node := range nodeList.Items {
		if err := CreateDRBDForNode(cli, &node, ownerReference); err != nil {
			log.Errorf("Create job err: %v", err)
			return err
		}
	}
	return nil
}

func CreateDRBDForNode(cli client.Client, node *corev1.Node, ownerReference []v1.OwnerReference) error {

	distro, needInstall := GetDistro(node)
	if needInstall {
		distroImageRepository = strings.Replace(shapperImageRepository, "shipper", distro, 1)
	} else {
		return nil
	}

	kernelVersion := node.Status.NodeInfo.KernelVersion
	jobName := "drbd-adapter-" + node.Name + "-" + distro
	newJobName := fmt.Sprintf("%s-%d", jobName, time.Now().Unix())

	job := batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      newJobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":          "drbd-adapter",
				"drbd-version": drbdVersion,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit0,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						"app":          "drbd-adapter",
						"drbd-version": drbdVersion,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": node.Name,
					},
					HostNetwork:                   true,
					HostPID:                       true,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds0,
					Containers: []corev1.Container{
						{
							Name:            "shipper",
							Image:           shapperImageRegistry + "/" + shapperImageRepository + ":" + drbdVersion + "_" + shipperChar,
							ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "pkgs",
									MountPath: "/pkgs",
								},
							},
						},
						{
							Name:            distro,
							Image:           shapperImageRegistry + "/" + distroImageRepository + ":" + tag + "_" + shipperChar,
							ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
							Command: []string{
								"/pkgs/entrypoint.adapter.sh",
								kernelVersion,
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &install.SecurityContextPrivilegedTrue,
							},
							Env: []corev1.EnvVar{
								{
									Name:  "LB_SKIP",
									Value: "no",
								},
								{
									Name:  "LB_DROP",
									Value: "yes",
								},
								{
									Name:  "LB_UPGRADE",
									Value: upgrade,
								},
								{
									Name:  "LB_CHECK_HOSTNAME",
									Value: checkHostName,
								},
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "pkgs",
									MountPath: "/pkgs",
								},
								{
									Name:      "pkgroot",
									MountPath: "/pkgs_root",
								},
								{
									Name:      "os-release",
									MountPath: "/etc/host-release",
									ReadOnly:  true,
								},
								{
									Name:      "usr-src",
									MountPath: "/usr/src",
									ReadOnly:  true,
								},
								{
									Name:      "lib-modules",
									MountPath: "/lib/modules",
								},
								{
									Name:      "usr-local-bin",
									MountPath: "/usr-local-bin",
								},
								{
									Name:      "etc-drbd-conf",
									MountPath: "/etc/drbd.conf",
								},
								{
									Name:      "etc-drbd-d",
									MountPath: "/etc/drbd.d",
								},
								{
									Name:      "var-lib-drbd",
									MountPath: "/var/lib/drbd",
									ReadOnly:  true,
								},
								{
									Name:      "etc-modules-load",
									MountPath: "/etc/modules-load.d",
								},
								{
									Name:      "etc-sysconfig-modules",
									MountPath: "/etc/sysconfig/modules",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "pkgs",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "pkgroot",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/root",
								},
							},
						},
						{
							Name: "os-release",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/os-release",
									Type: &install.HostPathFileOrCreate,
								},
							},
						},
						{
							Name: "centos-release",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/centos-release",
									Type: &install.HostPathFileOrCreate,
								},
							},
						},
						{
							Name: "usr-src",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/usr/src",
								},
							},
						},
						{
							Name: "lib-modules",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/lib/modules",
								},
							},
						},
						{
							Name: "usr-local-bin",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/usr/local/bin",
								},
							},
						},
						{
							Name: "etc-drbd-conf",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/drbd.conf",
									Type: &install.HostPathFileOrCreate,
								},
							},
						},
						{
							Name: "etc-drbd-d",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/drbd.d",
									Type: &install.HostPathDirectoryOrCreate,
								},
							},
						},
						{
							Name: "var-lib-drbd",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/drbd",
									Type: &install.HostPathDirectoryOrCreate,
								},
							},
						},
						{
							Name: "etc-modules-load",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/modules-load.d",
									Type: &install.HostPathDirectoryOrCreate,
								},
							},
						},
						{
							Name: "etc-sysconfig-modules",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/sysconfig/modules",
									Type: &install.HostPathDirectoryOrCreate,
								},
							},
						},
					},
				},
			},
		},
	}

	matched, err := regexp.Match("^rhel[78]$", []byte(distro))
	if err != nil {
		log.Errorf("Regexp match err: %v", err)
		return err
	}
	if matched {
		for i, container := range job.Spec.Template.Spec.Containers {
			if container.Name == distro {
				container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
					Name:      "centos-release",
					MountPath: "/etc/centos-release",
					ReadOnly:  true,
				})
				job.Spec.Template.Spec.Containers[i] = container
			}
		}
	}

	if useAffinity == "yes" {
		job.Spec.Template.Spec.Affinity.NodeAffinity = &nodeAffinity
	}

	if deployOnMaster {
		job.Spec.Template.Spec.Tolerations = []corev1.Toleration{
			{
				Effect:   corev1.TaintEffectNoSchedule,
				Key:      "node-role.kubernetes.io/master",
				Operator: corev1.TolerationOpExists,
			},
			{
				Effect:   corev1.TaintEffectNoSchedule,
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: corev1.TolerationOpExists,
			},
		}
	}
	job.OwnerReferences = ownerReference
	if err := cli.Create(context.TODO(), &job); err != nil {
		return err
	}

	return nil
}

func GetDistro(node *corev1.Node) (string, bool) {

	_, masterLabelExist := node.Labels["node-role.kubernetes.io/master"]
	_, controlPlaneLabel := node.Labels["node-role.kubernetes.io/control-plane"]
	if masterLabelExist || controlPlaneLabel {
		if !deployOnMaster {
			return "unsupported", false
		}
	}
	osImage := strings.ToLower(node.Status.NodeInfo.OSImage)
	distro := "unsupported"
	for k, v := range distroRegexMap {
		matched, err := regexp.Match(k, []byte(osImage))
		if err != nil {
			log.Errorf("Regexp match err: %v", err)
			return distro, false
		}
		if matched {
			distro = v
		}
		if distro == "jammy" {
			tag = "v9.1.11"
		}
	}
	if distro == "unsupported" {
		return distro, false
	}
	return distro, true
}

func FulfillDRBDSpec(clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.DRBD == nil {
		clusterInstance.Spec.DRBD = &hwameistoriov1alpha1.DRBDSpec{}
	}
	if clusterInstance.Spec.DRBD.DeployOnMaster == "" {
		clusterInstance.Spec.DRBD.DeployOnMaster = defaultDeployOnMaster
	}

	if clusterInstance.Spec.DRBD.Shipper == nil {
		clusterInstance.Spec.DRBD.Shipper = &operatorv1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.DRBD.Shipper.Registry == "" {
		clusterInstance.Spec.DRBD.Shipper.Registry = defaultImageRegistry
	}
	if clusterInstance.Spec.DRBD.Shipper.Repository == "" {
		clusterInstance.Spec.DRBD.Shipper.Repository = defaultShipperRepository
	}

	if clusterInstance.Spec.DRBD.Shipper.Tag == "" {
		clusterInstance.Spec.DRBD.Shipper.Tag = defaultShipperChar
	}

	if clusterInstance.Spec.DRBD.DRBDVersion == "" {
		clusterInstance.Spec.DRBD.DRBDVersion = defaultDRBDVersion
	}
	if clusterInstance.Spec.DRBD.Upgrade == "" {
		clusterInstance.Spec.DRBD.Upgrade = defaultDRBDUpgrade
	}
	if clusterInstance.Spec.DRBD.CheckHostName == "" {
		clusterInstance.Spec.DRBD.CheckHostName = defaultCheckHostName
	}
	if clusterInstance.Spec.DRBD.UseAffinity == "" {
		clusterInstance.Spec.DRBD.UseAffinity = defaultUseAffinity
	}
	if clusterInstance.Spec.DRBD.NodeAffinity == nil {
		clusterInstance.Spec.DRBD.NodeAffinity = &corev1.NodeAffinity{}
	}
	if clusterInstance.Spec.DRBD.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		clusterInstance.Spec.DRBD.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
	}
	if clusterInstance.Spec.DRBD.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms == nil {
		clusterInstance.Spec.DRBD.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = defaultNodeSelectTerms
	}
	if clusterInstance.Spec.DRBD.ChartVersion == "" {
		clusterInstance.Spec.DRBD.ChartVersion = defaultChartVersion
	}

	return clusterInstance
}
