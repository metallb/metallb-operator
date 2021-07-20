package controllers

import (
	"context"
	"github.com/metallb/metallb-operator/api/v1alpha1"
	"github.com/metallb/metallb-operator/test/consts"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var _ = Describe("AddressPool Controller", func() {
	Context("Creating AddressPool object", func() {
		autoAssign := false
		addressPool := &v1alpha1.AddressPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-addresspool",
				Namespace: MetalLBTestNameSpace,
			},
			Spec: v1alpha1.AddressPoolSpec{
				Protocol: "layer2",
				Addresses: []string{
					"1.1.1.1",
					"1.1.1.100",
				},
				AutoAssign: &autoAssign,
			},
		}

		AfterEach(func() {
			err := k8sClient.Delete(context.Background(), addressPool)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					Fail(err.Error())
				}
			}
			err = cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Should create AddressPool Object", func() {

			By("Creating a AddressPool resource")
			err := k8sClient.Create(context.Background(), addressPool)
			Expect(err).ToNot(HaveOccurred())

			// Checking ConfigMap is created
			By("By checking ConfigMap is created and matches test-addresspool configuration")
			Eventually(func() (string, error) {
				configmap := &corev1.ConfigMap{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: consts.MetalLBConfigMapName, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[consts.MetalLBConfigMapName], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`address-pools:
- name: test-addresspool
  protocol: layer2
  auto-assign: false
  addresses:

  - 1.1.1.1
  - 1.1.1.100

`))
		})
	})
})
