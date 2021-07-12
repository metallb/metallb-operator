/*
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

package platform

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var log = ctrl.Log.WithName("platform")

type k8SBasedPlatformVersioner struct{}

/*
GetPlatformInfo examines the Kubernetes-based environment and determines the running platform, version, & OS.
Accepts <nil> or instantiated 'cfg' rest config parameter.

Result: PlatformInfo{ Name: OpenShift, K8SVersion: 1.13+, OS: linux/amd64 }
*/
func GetPlatformInfo(cfg *rest.Config) (PlatformInfo, error) {
	return k8SBasedPlatformVersioner{}.getPlatformInfo(nil, cfg)
}

/*
GetPlatformName is a helper method to return the platform name from GetPlatformInfo results
Accepts <nil> or instantiated 'cfg' rest config parameter.
*/
func GetPlatformName(cfg *rest.Config) (string, error) {
	info, err := GetPlatformInfo(cfg)
	if err != nil {
		return "", err
	}
	return string(info.Name), nil
}

// deal with cfg coming from legacy method signature and allow injection for client testing
func (k8SBasedPlatformVersioner) defaultArgs(client discovery.DiscoveryInterface, cfg *rest.Config) (discovery.DiscoveryInterface, *rest.Config, error) {
	if cfg == nil {
		var err error
		cfg, err = config.GetConfig()
		if err != nil {
			return nil, nil, err
		}
	}
	if client == nil {
		var err error
		client, err = discovery.NewDiscoveryClientForConfig(cfg)
		if err != nil {
			return nil, nil, err
		}
	}
	return client, cfg, nil
}

func (pv k8SBasedPlatformVersioner) getPlatformInfo(client discovery.DiscoveryInterface, cfg *rest.Config) (PlatformInfo, error) {
	log.Info("detecting platform version...")
	info := PlatformInfo{Name: Kubernetes}

	var err error
	client, _, err = pv.defaultArgs(client, cfg)
	if err != nil {
		log.Info("issue occurred while defaulting client/cfg args")
		return info, err
	}

	k8sVersion, err := client.ServerVersion()
	if err != nil {
		log.Info("issue occurred while fetching ServerVersion")
		return info, err
	}
	info.K8SVersion = k8sVersion.Major + "." + k8sVersion.Minor
	info.OS = k8sVersion.Platform

	apiList, err := client.ServerGroups()
	if err != nil {
		log.Info("issue occurred while fetching ServerGroups")
		return info, err
	}

	for _, v := range apiList.Groups {
		if v.Name == "route.openshift.io" {

			log.Info("route.openshift.io found in apis, platform is OpenShift")
			info.Name = OpenShift
			break
		}
	}
	log.Info(info.String())
	return info, nil
}
