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

package helm

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type imageInfo struct {
	repo string
	tag  string
}

func parseManifest(manifest string) ([]*unstructured.Unstructured, error) {
	rendered := bytes.Buffer{}
	rendered.Write([]byte(manifest))
	out := []*unstructured.Unstructured{}
	// special case - if the entire file is whitespace, skip
	if len(strings.TrimSpace(rendered.String())) == 0 {
		return out, nil
	}

	decoder := yaml.NewYAMLOrJSONDecoder(&rendered, 4096)
	for {
		u := unstructured.Unstructured{}
		if err := decoder.Decode(&u); err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrapf(err, "failed to unmarshal manifest %s", manifest)
		}
		out = append(out, &u)
	}
	return out, nil
}

func setOcpMonitorFields(obj *unstructured.Unstructured) error {
	eps, found, err := unstructured.NestedSlice(obj.Object, "spec", "endpoints")
	if !found {
		return errors.New("failed to find endpoints in ServiceMonitor " + obj.GetName())
	}
	if err != nil {
		return err
	}
	for _, ep := range eps {
		err := unstructured.SetNestedField(ep.(map[string]interface{}), false, "tlsConfig", "insecureSkipVerify")
		if err != nil {
			return err
		}
	}
	err = unstructured.SetNestedSlice(obj.Object, eps, "spec", "endpoints")
	if err != nil {
		return err
	}
	return nil
}

func logLevelValue(crdConfig *metallbv1beta1.MetalLB) metallbv1beta1.MetalLBLogLevel {
	if crdConfig.Spec.LogLevel != "" {
		return crdConfig.Spec.LogLevel
	}
	return metallbv1beta1.LogLevelInfo
}

func toInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}

func valueWithDefault(name string, def int) (int, error) {
	val := os.Getenv(name)
	if val != "" {
		res, err := strconv.Atoi(val)
		if err != nil {
			return 0, err
		}
		return res, nil
	}
	return def, nil
}

func getImageNameTag(envValue string) (string, string) {
	pos := strings.LastIndex(envValue, "/")
	// We assume the last ":" shows up right before the image's tag, and the last "/" just before the image's name.
	// Multiple ":" can be present when the port of the registry is specified and we should include
	// it as part of the repo's url.
	img := strings.Split(envValue[pos+1:], ":")
	repoPath := envValue[:pos+1]

	if len(img) == 1 {
		return repoPath + img[0], ""
	}
	return repoPath + img[0], img[1]
}

func ocpPromConfigFor(component, namespace string) (map[string]interface{}, map[string]interface{}, string) {
	secretName := fmt.Sprintf("%s-certs-secret", component)

	annotations := map[string]interface{}{
		"service.beta.openshift.io/serving-cert-secret-name": secretName,
	}

	tlsConfig := map[string]interface{}{
		"caFile":             "/etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt",
		"serverName":         fmt.Sprintf("%s-monitor-service.%s.svc", component, namespace),
		"certFile":           "/etc/prometheus/secrets/metrics-client-certs/tls.crt",
		"keyFile":            "/etc/prometheus/secrets/metrics-client-certs/tls.key",
		"insecureSkipVerify": false,
	}

	return tlsConfig, annotations, secretName
}
