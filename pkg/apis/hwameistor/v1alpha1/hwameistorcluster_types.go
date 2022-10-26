package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HwameistorClusterSpec defines the desired state of HwameistorCluster
type HwameistorClusterSpec struct {
	// The global version information that instructs operator to orchestrate a particular version of Hwameistor.
	// This value can be overridden at the specific component level
	// +optional
	Image HwameistorImageSpec `json:"image"`

	// The version information that instructs operator to orchestrate a particular version of CSI.
	// +optional
	K8SImage K8SImageSpec `json:"k8SImage"`
}

// HwameistorImageSpec represents the settings for the Hwameistor version that operator is orchestrating.
type HwameistorImageSpec struct {
	// Registry indicates which registry to pull image from. e.x. ghcr.io
	// +optional
	Registry string `json:"registry"`

	// Tag represents image tag. e.x. v0.3.6
	// +optional
	Tag string `json:"imageTag"`
}

type K8SImageSpec struct {
	// Registry indicates which registry to pull K8s csi image from. The default is k8s.gcr.io
	// +optional
	Registry string `json:"registry,omitempty"`

	// csi-node-driver-registrar image
	Registrar ImageSpec `json:"registrar,omitempty"`

	// csi-attacher image
	Attacher ImageSpec `json:"attacher,omitempty"`

	// resizer image
	Resizer ImageSpec `json:"resizer,omitempty"`

	// provisioner image
	Provisioner ImageSpec `json:"provisioner,omitempty"`
}

// ImageSpec represents which image registry and image tag will be pulled from.
type ImageSpec struct {
	// Registry indicates which registry to pull image from. e.x. ghcr.io
	// +optional
	Registry string `json:"registry,omitempty"`

	// Repository indicates image name. e.x. hwameistor/local-storage
	Repository string `json:"repository,omitempty"`

	// Tag represents image tag. e.x. v0.3.6
	// +optional
	Tag string `json:"imageTag,omitempty"`
}

// StorageClassSpec is a storage class config
// +nullable
type StorageClassSpec struct {
	// Whether to create storageclass. Default to True.
	Disabled bool `json:"disabled,omitempty"`

	// Parameters holds the parameters for the provisioner that should
	// create volumes of this storage class.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// StorageClass describes the parameters for a class of storage for
	// which PersistentVolumes can be dynamically provisioned.
	// +nullable
	// +optional
	StorageClasses []v12.StorageClass `json:"storageClasses,omitempty"`
}

// LocalStorageSpec defines runtime config for localstorage
type LocalStorageSpec struct {
	// Image config for the local-storage pods
	Image ImageSpec `json:"image"`

	// The replicas num for the local-storage pods
	Replicas int `json:"replicas"`

	// PriorityClassName sets priority classes on components
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// +optional
	LivenessProbe *ProbeSpec `json:"livenessProbe,omitempty"`

	// +optional
	StartupProbe *ProbeSpec `json:"startupProbe,omitempty"`

	// The resource requirements for the local-storage pods
	// +kubebuilder:pruning:PreserveUnknownFields
	// +nullable
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`

	// The affinity to place the local-storage pods (default is to place on all available node) with a daemonset
	// +kubebuilder:pruning:PreserveUnknownFields
	// +nullable
	// +optional
	Placement Placement `json:"placement,omitempty"`

	// The annotations-related configuration to add/set on each Pod related object.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +nullable
	// +optional
	Annotations Annotations `json:"annotations,omitempty"`

	// The labels-related configuration to add/set on each Pod related object.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +nullable
	// +optional
	Labels Labels `json:"labels,omitempty"`
}

// Placement is the placement for an object
type Placement struct {
	// NodeAffinity is a group of node affinity scheduling rules
	// +optional
	NodeAffinity *v1.NodeAffinity `json:"nodeAffinity,omitempty"`
	// PodAffinity is a group of inter pod affinity scheduling rules
	// +optional
	PodAffinity *v1.PodAffinity `json:"podAffinity,omitempty"`
	// PodAntiAffinity is a group of inter pod anti affinity scheduling rules
	// +optional
	PodAntiAffinity *v1.PodAntiAffinity `json:"podAntiAffinity,omitempty"`
	// The pod this Toleration is attached to tolerates any taint that matches
	// the triple <key,value,effect> using the matching operator <operator>
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// TopologySpreadConstraint specifies how to spread matching pods among the given topology
	// +optional
	TopologySpreadConstraints []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// ProbeSpec is a wrapper around Probe so it can be enabled or disabled for a Ceph daemon
type ProbeSpec struct {
	// Disabled determines whether probe is disable or not
	// +optional
	Disabled bool `json:"disabled,omitempty"`
	// Probe describes a health check to be performed against a container to determine whether it is
	// alive or ready to receive traffic.
	// +optional
	Probe *v1.Probe `json:"probe,omitempty"`
}

// HwameistorClusterStatus defines the observed state of HwameistorCluster
type HwameistorClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HwameistorCluster is the Schema for the hwameistorclusters API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=hwameistorclusters,scope=Namespaced
type HwameistorCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HwameistorClusterSpec   `json:"spec,omitempty"`
	Status HwameistorClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HwameistorClusterList contains a list of HwameistorCluster
type HwameistorClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HwameistorCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HwameistorCluster{}, &HwameistorClusterList{})
}
