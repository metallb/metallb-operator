package controllers

import (
	"github.com/metallb/metallb-operator/test/consts"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

import (
	"context"
	"github.com/metallb/metallb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("BGPCommunity Controller", func() {
	Context("Creating BGPCommunity object", func() {
		configmap := &corev1.ConfigMap{}
		Peer := &v1alpha1.BGPCommunity{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-bgpcommunity",
				Namespace: MetalLBTestNameSpace,
			},
			Spec: v1alpha1.BGPCommunitySpec{
				BGPCommunity: map[string]string{
					"accept-sig": "7002:007",
				},
			},
		}

		AfterEach(func() {
			err := k8sClient.Delete(context.Background(), Peer)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					Fail(err.Error())
				}
			}
			err = k8sClient.Delete(context.Background(), configmap)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					Fail(err.Error())
				}
			}
			err = cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Should create BGPCommunity Object", func() {

			By("Creating a BGPCommunity resource")
			err := k8sClient.Create(context.Background(), Peer)
			Expect(err).ToNot(HaveOccurred())

			// Checking ConfigMap is created
			By("By checking ConfigMap is created and matches test-bgpcommunity configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(),
					types.NamespacedName{Name: consts.MetalLBConfigMapName, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[consts.MetalLBConfigMapName], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`bgp-communities:
          accept-sig: 7002:007
`))
		})
	})
})
