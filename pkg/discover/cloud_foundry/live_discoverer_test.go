package cloud_foundry

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

var _ = Describe("writeToYAMLFile tests", func() {
	var tempFilename string
	BeforeEach(func() {
		tmpfile, err := os.CreateTemp("", "")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(tmpfile.Name())
		tempFilename = tmpfile.Name()
	})

	Describe("successful writes", func() {
		It("writes a string value correctly", func() {
			input := "test"
			Expect(writeToYAMLFile(input, tempFilename)).To(Succeed())

			var result string
			verifyYAMLFile(tempFilename, &result)
			Expect(result).To(Equal(input))
		})

		It("writes an empty value", func() {
			Expect(writeToYAMLFile("", tempFilename)).To(Succeed())

			_, err := os.Stat(tempFilename)
			Expect(err).ToNot(HaveOccurred())
		})
		It("writes a nil value", func() {
			Expect(writeToYAMLFile(nil, tempFilename)).To(Succeed())

			_, err := os.Stat(tempFilename)
			Expect(err).ToNot(HaveOccurred())
		})
		It("writes structured data correctly", func() {
			input := map[string]string{"key": "value"}
			Expect(writeToYAMLFile(input, tempFilename)).To(Succeed())

			var result map[string]string
			verifyYAMLFile(tempFilename, &result)
			Expect(result).To(Equal(input))
		})
		It("creates the file if doesn't exist", func() {
			tmpdir, err := os.MkdirTemp("", "")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir)
			Expect(writeToYAMLFile("test", tmpdir+"/test.yaml")).To(Succeed())
		})
	})
})

// Helper function
func verifyYAMLFile(filename string, target interface{}) {
	data, err := os.ReadFile(filename)
	Expect(err).NotTo(HaveOccurred())
	Expect(yaml.Unmarshal(data, target)).To(Succeed())
}
