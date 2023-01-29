package drbd

import (
	"context"
	"regexp"
	"strings"

	"github.com/hwameistor/hwameistor-operator/pkg/install"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
)

var distroRegexMap = map[string]string{
	"(red hat enterprise|centos|almalinux|rocky linux) .*7": "rhel7",
	"(red hat enterprise|centos|almalinux|rocky linux) .*8": "rhel8",
	"(red hat enterprise|centos|almalinux|rocky linux) .*9": "rhel9",
	"ubuntu .*18": "bionic",
	"ubuntu .*20": "focal",
	"ubuntu .*22": "jammy",
	"kylin .*v10": "kylin10",
}

var ttlSecondsAfterFinished3600 = int32(3600)
var backoffLimit0 = int32(0)
var terminationGracePeriodSeconds0 = int64(0)

var deployOnMaster = false
var drbdVersion string
var tag string
var imagePullPolicy string
var imageRegistry string
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
	imageRegistry = drbdConfigs.ImageRegistry
	imagePullPolicy = drbdConfigs.ImagePullPolicy
	chartVersion = drbdConfigs.ChartVersion
	upgrade = drbdConfigs.Upgrade
	checkHostName = drbdConfigs.CheckHostName
	useAffinity = drbdConfigs.UseAffinity
	nodeAffinity = drbdConfigs.NodeAffinity
}

func CreateDRBDAdapter(cli client.Client) error {
	nodeList := corev1.NodeList{}
	if err := cli.List(context.TODO(), &nodeList); err != nil {
		log.Errorf("List nodes err: %v", err)
		return err
	}

	for _, node := range nodeList.Items {
		_, masterLabelExist := node.Labels["node-role.kubernetes.io/master"]
		_, controlPlaneLabel := node.Labels["node-role.kubernetes.io/control-plane"]
		if masterLabelExist || controlPlaneLabel {
			if !deployOnMaster {
				continue
			}
		}

		osImage := strings.ToLower(node.Status.NodeInfo.OSImage)
		distro := "unsupported"
		for k, v := range distroRegexMap {
			matched, err := regexp.Match(k, []byte(osImage))
			if err != nil {
				log.Errorf("Regexp match err: %v", err)
				return err
			}
			if matched {
				distro = v
			}
			if distro == "jammy" {
				tag = "v9.1.11"
			}
		}
		if distro == "unsupported" {
			continue
		}

		job := batchv1.Job{
			ObjectMeta: v1.ObjectMeta{
				Name: "drbd-adapter-" + node.Name + "-" + distro,
				Namespace: namespace,
				Labels: map[string]string{
					"app": "drbd-adapter",
					"drbd-version": drbdVersion,
				},
			},
			Spec: batchv1.JobSpec{
				TTLSecondsAfterFinished: &ttlSecondsAfterFinished3600,
				BackoffLimit: &backoffLimit0,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: v1.ObjectMeta{
						Labels: map[string]string{
							"app": "drbd-adapter",
							"drbd-version": drbdVersion,
						},
					},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						NodeSelector: map[string]string{
							"kubernetes.io/hostname": node.Name,
						},
						HostNetwork: true,
						TerminationGracePeriodSeconds: &terminationGracePeriodSeconds0,
						Containers: []corev1.Container{
							{
								Name: "shipper",
								Image: imageRegistry + "/" + "drbd9-shipper" + ":" + drbdVersion+"_"+chartVersion,
								ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
								VolumeMounts: []corev1.VolumeMount{
									{
										Name: "pkgs",
										MountPath: "/pkgs",
									},
								},
							},
							{
								Name: distro,
								Image: imageRegistry + "/" + "drbd9"+"-"+distro + ":" + tag,
								ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
								Command: []string{
									"/pkgs/entrypoint.adapter.sh",
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged: &install.SecurityContextPrivilegedTrue,
								},
								Env: []corev1.EnvVar{
									{
										Name: "LB_SKIP",
										Value: "no",
									},
									{
										Name: "LB_DROP",
										Value: "yes",
									},
									{
										Name: "LB_UPGRADE",
										Value: upgrade,
									},
									{
										Name: "LB_CHECK_HOSTNAME",
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
										Name: "pkgs",
										MountPath: "/pkgs",
									},
									{
										Name: "os-release",
										MountPath: "/etc/host-release",
										ReadOnly: true,
									},
									{
										Name: "usr-src",
										MountPath: "/usr/src",
										ReadOnly: true,
									},
									{
										Name: "lib-modules",
										MountPath: "/lib/modules",
									},
									{
										Name: "usr-local-bin",
										MountPath: "/usr-local-bin",
									},
									{
										Name: "etc-drbd-conf",
										MountPath: "/etc/drbd.conf",
									},
									{
										Name: "etc-drbd-d",
										MountPath: "/etc/drbd.d",
									},
									{
										Name: "var-lib-drbd",
										MountPath: "/var/lib/drbd",
										ReadOnly: true,
									},
									{
										Name: "etc-modules-load",
										MountPath: "/etc/modules-load.d",
									},
									{
										Name: "etc-sysconfig-modules",
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
			for _, container := range job.Spec.Template.Spec.Containers {
				if container.Name == distro {
					container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
						Name: "centos-release",
						MountPath: "/etc/centos-release",
						ReadOnly: true,
					})
				}
			}
		}

		if useAffinity == "yes" {
			job.Spec.Template.Spec.Affinity.NodeAffinity = &nodeAffinity
		}

		if err := cli.Create(context.TODO(), &job); err != nil {
			log.Errorf("Create job err: %v", job)
			return err
		}
	}

	return nil
}