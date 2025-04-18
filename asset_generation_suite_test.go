package asset_generation_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAssetGeneration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AssetGeneration Suite")
}
