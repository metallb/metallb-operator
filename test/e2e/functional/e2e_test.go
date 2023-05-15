//go:build e2etests
// +build e2etests

package functional

import (
	"flag"
	"os"
	"path"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"

	"github.com/metallb/metallb-operator/test/consts"
	_ "github.com/metallb/metallb-operator/test/e2e/functional/tests"
	"github.com/metallb/metallb-operator/test/e2e/k8sreporter"
	kniK8sReporter "github.com/openshift-kni/k8sreporter"
)

var (
	OperatorNameSpace = consts.DefaultOperatorNameSpace
	junitPath         *string
	reportPath        *string
	r                 *kniK8sReporter.KubernetesReporter
)

func init() {
	if ns := os.Getenv("OO_INSTALL_NAMESPACE"); len(ns) != 0 {
		OperatorNameSpace = ns
	}

	junitPath = flag.String("junit", "", "the path for the junit format report")
	reportPath = flag.String("report", "", "the path of the report file containing details for failed tests")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	_, reporterConfig := GinkgoConfiguration()

	if *reportPath != "" {
		kubeconfig := os.Getenv("KUBECONFIG")
		r = k8sreporter.New(kubeconfig, *reportPath, OperatorNameSpace)
	}

	RunSpecs(t, "Metallb Operator E2E Suite", reporterConfig)
}

var _ = ReportAfterSuite("e2esuite", func(report types.Report) {
	if *junitPath != "" {
		junitFile := path.Join(*junitPath, "metallb_operator_e2e_junit.xml")
		reporters.GenerateJUnitReportWithConfig(report, junitFile, reporters.JunitReportConfig{
			OmitTimelinesForSpecState: types.SpecStatePassed | types.SpecStateSkipped,
			OmitLeafNodeType:          true,
			OmitSuiteSetupNodes:       true,
		})
	}
})

var _ = ReportAfterEach(func(specReport types.SpecReport) {
	if specReport.Failed() == false {
		return
	}

	if *reportPath != "" {
		k8sreporter.DumpInfo(r, specReport.FullText())
	}
})
