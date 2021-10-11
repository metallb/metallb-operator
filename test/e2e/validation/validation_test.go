//go:build validationtests
// +build validationtests

package validation

import (
	"flag"
	"os"
	"path"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	"github.com/metallb/metallb-operator/test/consts"
	testclient "github.com/metallb/metallb-operator/test/e2e/client"
	"github.com/metallb/metallb-operator/test/e2e/k8sreporter"
	_ "github.com/metallb/metallb-operator/test/e2e/validation/tests"
)

var OperatorNameSpace = consts.DefaultOperatorNameSpace

var junitPath *string
var reportPath *string

func init() {
	if ns := os.Getenv("OO_INSTALL_NAMESPACE"); len(ns) != 0 {
		OperatorNameSpace = ns
	}

	junitPath = flag.String("junit", "", "the path for the junit format report")
	reportPath = flag.String("report", "", "the path of the report file containing details for failed tests")
}

func TestValidation(t *testing.T) {
	RegisterFailHandler(Fail)

	rr := []Reporter{}
	if *junitPath != "" {
		junitFile := path.Join(*junitPath, "validation_junit.xml")
		rr = append(rr, reporters.NewJUnitReporter(junitFile))
	}

	clients := testclient.New("")

	if *reportPath != "" {
		rr = append(rr, k8sreporter.New(clients, OperatorNameSpace, *reportPath))
	}

	RunSpecsWithDefaultAndCustomReporters(t, "Metallb Operator Validation Suite", rr)
}
