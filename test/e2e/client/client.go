package client

import (
	"os"

	"github.com/golang/glog"
	metallbv1alpha "github.com/metallb/metallb-operator/api/v1alpha1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	discovery "k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	networkv1client "k8s.io/client-go/kubernetes/typed/networking/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client defines the client set that will be used for testing
var Client *ClientSet

func init() {
	Client = New("")
}

// ClientSet provides the struct to talk with relevant API
type ClientSet struct {
	client.Client
	corev1client.CoreV1Interface
	networkv1client.NetworkingV1Client
	appsv1client.AppsV1Interface
	rbacv1client.RbacV1Interface
	discovery.DiscoveryInterface
	Config *rest.Config
}

// New returns a *ClientBuilder with the given kubeconfig.
func New(kubeconfig string) *ClientSet {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	if kubeconfig != "" {
		glog.V(4).Infof("Loading kube client config from path %q", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		glog.V(4).Infof("Using in-cluster kube client config")
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Infof("Failed to init kubernetes client, please check the $KUBECONFIG environment variable")
		return nil
	}

	clientSet := &ClientSet{}
	clientSet.CoreV1Interface = corev1client.NewForConfigOrDie(config)
	clientSet.AppsV1Interface = appsv1client.NewForConfigOrDie(config)
	clientSet.RbacV1Interface = rbacv1client.NewForConfigOrDie(config)
	clientSet.DiscoveryInterface = discovery.NewDiscoveryClientForConfigOrDie(config)
	clientSet.NetworkingV1Client = *networkv1client.NewForConfigOrDie(config)
	clientSet.Config = config

	myScheme := runtime.NewScheme()
	if err = scheme.AddToScheme(myScheme); err != nil {
		panic(err)
	}

	// Setup Scheme for all resources
	if err := apiext.AddToScheme(myScheme); err != nil {
		panic(err)
	}

	if err := metallbv1alpha.AddToScheme(myScheme); err != nil {
		panic(err)
	}

	clientSet.Client, err = client.New(config, client.Options{
		Scheme: myScheme,
	})

	if err != nil {
		return nil
	}

	return clientSet
}
