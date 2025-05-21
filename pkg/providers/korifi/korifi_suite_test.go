package korifi_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKorifi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Korifi Suite")
}
