package rbac

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)
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

func InstallRBAC(cli client.Client) error {
	if err := cli.Create(context.TODO(), &clusterRole); err != nil {
		log.Errorf("Create ClusterRole err: %v", err)
		return err
	}

	if err := cli.Create(context.TODO(), &sa); err != nil {
		log.Errorf("Create ServiceAccount err: %v", err)
		return err
	}

	if err := cli.Create(context.TODO(), &clusterRoleBinding); err != nil {
		log.Errorf("Create ClusterRoleBinding err: %v", err)
		return err
	}

	return nil
}