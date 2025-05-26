package cloud_foundry_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCF(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cloud Foundry Suite")
}
