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
	"io"
	"strings"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	bgpFrr = "frr"
)

// MetalLBChart metallb chart struct containing references which helps to
// to retrieve manifests from chart after patching given custom values.
type MetalLBChart struct {
	client      *action.Install
	envSettings *cli.EnvSettings
	chart       *chart.Chart
	config      *chartConfig
	namespace   string
}

// GetObjects retrieve manifests from chart after patching custom values passed in crdConfig
// and environment variables.
func (h *MetalLBChart) GetObjects(crdConfig *metallbv1beta1.MetalLB) ([]*unstructured.Unstructured, error) {
	chartValueOpts := &values.Options{}
	chartValues, err := chartValueOpts.MergeValues(getter.All(h.envSettings))
	if err != nil {
		return nil, err
	}

	patchToChartValues(h.config, crdConfig, chartValues)

	release, err := h.client.Run(h.chart, chartValues)
	if err != nil {
		return nil, err
	}
	objs, err := parseManifest(release.Manifest)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		// Set namespace explicitly into non cluster-scoped resource because helm doesn't
		// patch namespace into manifests at client.Run.
		objKind := obj.GetKind()
		if objKind != "PodSecurityPolicy" {
			obj.SetNamespace(h.namespace)
		}
	}
	return objs, nil
}

// InitMetalLBChart initializes metallb helm chart after loading it from given
// chart path and creating config object from environment variables.
func InitMetalLBChart(chartPath, chartName, namespace string,
	client client.Client, isOpenshift bool) (*MetalLBChart, error) {
	chart := &MetalLBChart{}
	chart.namespace = namespace
	chart.envSettings = cli.New()
	chart.client = action.NewInstall(new(action.Configuration))
	chart.client.ReleaseName = chartName
	chart.client.DryRun = true
	chart.client.ClientOnly = true
	chartPath, err := chart.client.ChartPathOptions.LocateChart(chartPath, chart.envSettings)
	if err != nil {
		return nil, err
	}
	chart.chart, err = loader.Load(chartPath)
	if err != nil {
		return nil, err
	}
	chart.config, err = loadConfig(client, isOpenshift)
	if err != nil {
		return nil, err
	}
	return chart, nil
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
