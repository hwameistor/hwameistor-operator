package E2eTest

import (
	"context"
	"time"

	"github.com/hwameistor/hwameistor-operator/test/e2e/framework"
	"github.com/hwameistor/hwameistor-operator/test/e2e/utils"
	clientset "github.com/hwameistor/hwameistor/pkg/apis/client/clientset/versioned/scheme"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = ginkgo.Describe("localstorage volume test ", ginkgo.Label("periodCheck"), func() {

	var f *framework.Framework
	var client ctrlclient.Client
	ctx := context.TODO()
	ginkgo.It("Configure the base environment", func() {
		logrus.Info("start rollback")
		_ = utils.RunInLinux("sh rollback.sh")
		err := wait.PollImmediate(10*time.Second, 20*time.Minute, func() (done bool, err error) {
			output := utils.RunInLinux("kubectl get pod -A  |grep -v Running |wc -l")
			if output != "1\n" {
				return false, nil
			} else {
				logrus.Info("k8s ready")
				return true, nil
			}

		})
		if err != nil {
			logrus.Error(err)
		}

		f = framework.NewDefaultFramework(clientset.AddToScheme)
		client = f.GetClient()

	})
	ginkgo.It("install hwameistor-operator", func() {
		err := utils.InstallHwameistorOperator(ctx, client)
		gomega.Expect(err).To(gomega.BeNil())

	})
	ginkgo.Context("create a hmcluster", func() {
		ginkgo.It("create a hmcluster", func() {
			//create sc
			//exampleCluster := &opv1.Cluster{
			//	TypeMeta: metav1.TypeMeta{
			//		Kind:       "Cluster",
			//		APIVersion: "hwameistor.io/v1alpha1",
			//	},
			//	ObjectMeta: metav1.ObjectMeta{
			//		Name: "cluster-sample",
			//	},
			//	Spec:   opv1.ClusterSpec{},
			//	Status: opv1.ClusterStatus{},
			//}
			//
			//err := client.Create(ctx, exampleCluster)
			//if err != nil {
			//	logrus.Printf("Create hmcluster failed ：%+v ", err)
			//	f.ExpectNoError(err)
			//}
			//_ = utils.RunInLinux("kubectl apply -f sample.yaml")
			time.Sleep(1 * time.Minute)
			err := utils.CheckHwameiInstall(ctx)
			gomega.Expect(err).To(gomega.BeNil())
		})
	})

})
