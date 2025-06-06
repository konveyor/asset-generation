package cloud_foundry

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var GlobalT *testing.T

func TestCloudFoundry(t *testing.T) {
	GlobalT = t // save the *testing.T pointer globally
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cloud Foundry Suite")
}
