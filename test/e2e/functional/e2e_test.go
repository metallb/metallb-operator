//go:build e2etests
// +build e2etests

package functional

import (
	"flag"
	"os"
	"path"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/metallb/metallb-operator/test/consts"
	_ "github.com/metallb/metallb-operator/test/e2e/functional/tests"
	"github.com/metallb/metallb-operator/test/e2e/k8sreporter"
	kniK8sReporter "github.com/openshift-kni/k8sreporter"
)

var OperatorNameSpace = consts.DefaultOperatorNameSpace

var junitPath *string
var reportPath *string
var r *kniK8sReporter.KubernetesReporter

func init() {
	if ns := os.Getenv("OO_INSTALL_NAMESPACE"); len(ns) != 0 {
		OperatorNameSpace = ns
	}

	junitPath = flag.String("junit", "", "the path for the junit format report")
	reportPath = flag.String("report", "", "the path of the report file containing details for failed tests")
}

func TestE2E(t *testing.T) {
	// We want to collect logs before any resource is deleted in AfterEach, so we register the global fail handler
	// in a way such that the reporter's Dump is always called before the default Fail.
	RegisterFailHandler(func(message string, callerSkip ...int) {
		if r != nil {
			r.Dump(consts.LogsExtractDuration, CurrentSpecReport().FullText())
		}

		// Ensure failing line location is not affected by this wrapper
		for i := range callerSkip {
			callerSkip[i]++
		}
		Fail(message, callerSkip...)
	})

	_, reporterConfig := GinkgoConfiguration()

	if *junitPath != "" {
		junitFile := path.Join(*junitPath, "e2e_junit.xml")
		reporterConfig.JUnitReport = junitFile
	}

	if *reportPath != "" {
		kubeconfig := os.Getenv("KUBECONFIG")
		reportPath := path.Join(*reportPath, "metallb_failure_report.log")
		r = k8sreporter.New(kubeconfig, reportPath, OperatorNameSpace)
	}

	RunSpecs(t, "Metallb Operator E2E Suite", reporterConfig)
}
