package k8sreporter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/kennygrant/sanitize"
	testclient "github.com/metallb/metallb-operator/test/e2e/client"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var operatorNameSpace string

type KubernetesReporter struct {
	sync.Mutex
	clients    *testclient.ClientSet
	reportPath string
}

func New(clients *testclient.ClientSet, nameSpace string, reportPath string) *KubernetesReporter {
	operatorNameSpace = nameSpace
	return &KubernetesReporter{clients: clients, reportPath: reportPath}
}

func (r *KubernetesReporter) SpecSuiteWillBegin(config config.GinkgoConfigType, summary *types.SuiteSummary) {

}

func (r *KubernetesReporter) BeforeSuiteDidRun(setupSummary *types.SetupSummary) {
	r.Cleanup()
}

func (r *KubernetesReporter) SpecWillRun(specSummary *types.SpecSummary) {
}

func (r *KubernetesReporter) SpecDidComplete(specSummary *types.SpecSummary) {
	r.Lock()
	defer r.Unlock()

	if !specSummary.HasFailureState() {
		return
	}
	f, err := logFileFor(r.reportPath, "all", "")
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, "Starting dump for failed spec", specSummary.ComponentTexts)
	dirName := sanitize.BaseName(strings.Join(specSummary.ComponentTexts, ""))
	dirName = strings.Replace(dirName, "Top-Level", "", 1)
	r.Dump(dirName)
	fmt.Fprintln(f, "Finished dump for failed spec")
}

func (r *KubernetesReporter) Dump(dirName string) {
	err := os.Mkdir(path.Join(r.reportPath, dirName), 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create test dir: %v\n", err)
		return
	}

	r.logNodes(dirName)
	r.logPods(operatorNameSpace, dirName)
	r.logLogs(func(p *corev1.Pod) bool {
		return !strings.Contains(p.Namespace, "metallb")
	}, dirName)

}

// Cleanup cleans up the current content of the artifactsDir
func (r *KubernetesReporter) Cleanup() {
}

func (r *KubernetesReporter) logPods(namespace string, dirName string) {
	pods, err := r.clients.Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pods: %v\n", err)
		return
	}
	for _, pod := range pods.Items {
		f, err := logFileFor(r.reportPath, dirName, pod.Namespace+"-pods_specs")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open pods_specs file: %v\n", dirName)
			return
		}
		defer f.Close()
		fmt.Fprintf(f, "-----------------------------------\n")
		j, err := json.MarshalIndent(pod, "", "    ")
		if err != nil {
			fmt.Println("Failed to marshal pods", err)
			return
		}
		fmt.Fprintln(f, string(j))
	}
}

func (r *KubernetesReporter) logNodes(dirName string) {
	f, err := logFileFor(r.reportPath, dirName, "nodes")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open nodes file: %v\n", dirName)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "-----------------------------------\n")

	nodes, err := r.clients.Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch nodes: %v\n", err)
		return
	}

	j, err := json.MarshalIndent(nodes, "", "    ")
	if err != nil {
		fmt.Println("Failed to marshal nodes")
		return
	}
	fmt.Fprintln(f, string(j))
}

func (r *KubernetesReporter) logLogs(filterPods func(*corev1.Pod) bool, dirName string) {
	pods, err := r.clients.Pods(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pods: %v\n", err)
		return
	}
	for _, pod := range pods.Items {
		if filterPods(&pod) {
			continue
		}
		f, err := logFileFor(r.reportPath, dirName, pod.Namespace+"-pods_logs")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open pods_logs file: %v\n", dirName)
			return
		}
		defer f.Close()
		containersToLog := make([]v1.Container, 0)
		containersToLog = append(containersToLog, pod.Spec.Containers...)
		containersToLog = append(containersToLog, pod.Spec.InitContainers...)
		for _, container := range containersToLog {
			logs, err := r.clients.Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{Container: container.Name}).DoRaw(context.Background())
			if err == nil {
				fmt.Fprintf(f, "-----------------------------------\n")
				fmt.Fprintf(f, "Dumping logs for pod %s-%s-%s\n", pod.Namespace, pod.Name, container.Name)
				fmt.Fprintln(f, string(logs))
			}
		}

	}
}

func (r *KubernetesReporter) AfterSuiteDidRun(setupSummary *types.SetupSummary) {

}

func (r *KubernetesReporter) SpecSuiteDidEnd(summary *types.SuiteSummary) {

}

func logFileFor(dirName string, testName string, kind string) (*os.File, error) {
	path := path.Join(dirName, testName, kind) + ".log"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return f, nil
}
