package rbac

import (
	"context"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RBACMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

var defaultServiceAccountName = "hwameistor-admin"

func NewMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *RBACMaintainer {
	return &RBACMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var clusterRole = rbacv1.ClusterRole{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-role",
	},
	Rules: []rbacv1.PolicyRule{
		{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs: []string{"*"},
		},
		{
			NonResourceURLs: []string{"*"},
			Verbs: []string{"*"},
		},
	},
}

var sa = corev1.ServiceAccount{
	ObjectMeta: metav1.ObjectMeta{},
}

var clusterRoleBinding = rbacv1.ClusterRoleBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-admin-binding",
	},
	RoleRef: rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind: "ClusterRole",
		Name: "hwameistor-role",
	},
	Subjects: []rbacv1.Subject{
		{
			Kind: "ServiceAccount",
		},
	},
}

func setServiceAccount(namespace, name string) {
	sa.Namespace = namespace
	sa.Name = name
}

func setClusterRoleBinding(subjectNamespace, subjectName string) {
	clusterRoleBinding.Subjects[0].Name = subjectName
	clusterRoleBinding.Subjects[0].Namespace = subjectNamespace
}

func SetRBAC(clusterInstance *hwameistoriov1alpha1.Cluster) {
	setServiceAccount(clusterInstance.Spec.TargetNamespace, clusterInstance.Spec.RBAC.ServiceAccountName)
	setClusterRoleBinding(clusterInstance.Spec.TargetNamespace, clusterInstance.Spec.RBAC.ServiceAccountName)
}

func (m *RBACMaintainer) Ensure() error {
	SetRBAC(m.ClusterInstance)
	if err := m.ensureClusterRole(); err != nil {
		log.Errorf("ensure ClusterRole err: %v", err)
		return err
	}
	if err := m.ensureServiceAccount(); err != nil {
		log.Errorf("ensure ServiceAccount err: %v", err)
		return err
	}
	if err := m.ensureClusterRoleBinding(); err != nil {
		log.Errorf("ensure ClusterRoleBinding err: %v", err)
		return err
	}

	return nil
}

func (m *RBACMaintainer) ensureClusterRole() error {
	key := types.NamespacedName{
		Name: clusterRole.Name,
	}
	var gottenClusterRole rbacv1.ClusterRole
	if err := m.Client.Get(context.TODO(), key, &gottenClusterRole); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &clusterRole); errCreate != nil {
				log.Errorf("Create ClusterRole err: %v", err)
				return errCreate
			}
		} else {
			log.Errorf("Get ClusterRole err: %v", err)
			return err
		}
	}
	
	return nil
}

func (m *RBACMaintainer) ensureServiceAccount() error {
	key := types.NamespacedName{
		Namespace: sa.Namespace,
		Name: sa.Name,
	}
	var gottenSA corev1.ServiceAccount
	if err := m.Client.Get(context.TODO(), key, &gottenSA); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &sa); errCreate != nil {
				log.Errorf("Create ServiceAccount err: %v", errCreate)
				return errCreate
			}
		} else {
			log.Errorf("Get ServiceAccount err: %v", err)
			return err
		}
	}

	return nil
}

func (m *RBACMaintainer) ensureClusterRoleBinding() error {
	key := types.NamespacedName{
		Name: clusterRoleBinding.Name,
	}
	var gottenClusterRoleBinding rbacv1.ClusterRoleBinding
	if err := m.Client.Get(context.TODO(), key, &gottenClusterRoleBinding); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &clusterRoleBinding); errCreate != nil {
				log.Errorf("Create ClusterRoleBinding err: %v", errCreate)
				return errCreate
			}
		} else {
			log.Errorf("Get ClusterRoleBinding err: %v", err)
			return err
		}
	}

	return nil
}

func FulfillRBACSpec (clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.RBAC == nil {
		clusterInstance.Spec.RBAC = &hwameistoriov1alpha1.RBACSpec{}
	}
	if clusterInstance.Spec.RBAC.ServiceAccountName == "" {
		clusterInstance.Spec.RBAC.ServiceAccountName = defaultServiceAccountName
	}

	return clusterInstance
}