package helm_test

import (
	"maps"
	"os"
	"path"

	"github.com/konveyor/asset-generation/pkg/providers/helm"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

var _ = Describe("Helm command", func() {

	const (
		testDiscoverPath = "./test_data/discover.yaml"
		chartDir         = "./test_data/"
	)

	When("validating the execution when generating templates", func() {

		DescribeTable("when generating the manifests", func(cfg helm.Config, additionalValues map[string]any, expectedManifests map[string]string) {
			values := loadValues(testDiscoverPath, additionalValues)
			cfg.Values = values
			generator := helm.New(cfg)
			generatedManifests, err := generator.Generate()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(generatedManifests)).To(Equal(len(expectedManifests)))
			for k, expectedManifest := range expectedManifests {
				gManifest, ok := generatedManifests[k]
				Expect(ok).To(BeTrue())
				Expect(gManifest).To(Equal(expectedManifest))
			}

		},
			Entry("generates the manifests for a K8s chart using the discover manifest as input",
				helm.Config{
					ChartPath:                 path.Join(chartDir, "k8s_only"),
					SkipRenderNonK8SManifests: true,
				}, nil, map[string]string{"k8s_only/templates/configmap.yaml": `apiVersion: v1
data:
  chartName: hello world!
kind: ConfigMap
metadata:
  name: sample
`}),
			Entry("generates the manifests for a K8s chart while overriding the variable in the discover.yaml",
				helm.Config{
					ChartPath:                 path.Join(chartDir, "k8s_only"),
					SkipRenderNonK8SManifests: true,
				}, map[string]any{"foo": map[string]any{"bar": "bar.foo"}},
				map[string]string{"k8s_only/templates/configmap.yaml": `apiVersion: v1
data:
  chartName: bar.foo
kind: ConfigMap
metadata:
  name: sample
`}),
			Entry("generates the manifests for a K8s chart while adding a new variable that is interpreted in the template",
				helm.Config{
					ChartPath:                 path.Join(chartDir, "k8s_only"),
					SkipRenderNonK8SManifests: true,
				}, map[string]any{"extra": map[string]any{"value": "Lorem Ipsum"}},
				map[string]string{"k8s_only/templates/configmap.yaml": `apiVersion: v1
data:
  chartName: hello world!
  extraValue: Lorem Ipsum
kind: ConfigMap
metadata:
  name: sample
`}),
			Entry("generates no manifest in a K8s chart when specifying the flag to generate only the non-K8s templates",
				helm.Config{
					ChartPath:              path.Join(chartDir, "k8s_only"),
					SkipRenderK8SManifests: true,
				}, nil, map[string]string{}),
			Entry("generates both non-K8s and K8s manifests in a chart that contains both type of templates with the discover manifest as input",
				helm.Config{
					ChartPath: path.Join(chartDir, "mixed_templates"),
				}, nil, map[string]string{"mixed_templates/templates/configmap.yaml": `apiVersion: v1
data:
  chartName: hello world!
kind: ConfigMap
metadata:
  name: sample
`,
					"mixed_templates/files/konveyor/Dockerfile": `FROM python:3

RUN echo hello world!`}),
			Entry("with a chart with mixed templates and overriding the variable in the values.yaml",
				helm.Config{
					ChartPath: path.Join(chartDir, "mixed_templates"),
				}, map[string]any{"foo": map[string]any{"bar": "bar.foo"}},
				map[string]string{"mixed_templates/templates/configmap.yaml": `apiVersion: v1
data:
  chartName: bar.foo
kind: ConfigMap
metadata:
  name: sample
`, "mixed_templates/files/konveyor/Dockerfile": `FROM python:3

RUN echo bar.foo`}),
			Entry("with a chart with mixed templates and adding a new variable that is captured in the template",
				helm.Config{
					ChartPath: path.Join(chartDir, "mixed_templates"),
				}, map[string]any{"extra": map[string]any{"value": "Lorem Ipsum"}},
				map[string]string{"mixed_templates/templates/configmap.yaml": `apiVersion: v1
data:
  chartName: hello world!
  extraValue: Lorem Ipsum
kind: ConfigMap
metadata:
  name: sample
`, "mixed_templates/files/konveyor/Dockerfile": `FROM python:3

RUN echo hello world!
RUN echo Lorem Ipsum
`}),
			Entry("with a chart with mixed templates with multiple variables as input",
				helm.Config{
					ChartPath: path.Join(chartDir, "mixed_templates"),
				}, map[string]any{"extra": map[string]any{"value": "Lorem Ipsum"}, "foo": map[string]any{"bar": "bar foo"}},
				map[string]string{"mixed_templates/templates/configmap.yaml": `apiVersion: v1
data:
  chartName: bar foo
  extraValue: Lorem Ipsum
kind: ConfigMap
metadata:
  name: sample
`, "mixed_templates/files/konveyor/Dockerfile": `FROM python:3

RUN echo bar foo
RUN echo Lorem Ipsum
`}),
			Entry("only generates the non-K8s manifests in a chart that contains both type of templates",
				helm.Config{
					ChartPath:              path.Join(chartDir, "mixed_templates"),
					SkipRenderK8SManifests: true,
				}, nil, map[string]string{"mixed_templates/files/konveyor/Dockerfile": `FROM python:3

RUN echo hello world!`}),
			Entry("Skip generating both non and K8s manifests",
				helm.Config{
					ChartPath:                 path.Join(chartDir, "mixed_templates"),
					SkipRenderK8SManifests:    true,
					SkipRenderNonK8SManifests: true,
				}, nil, map[string]string{}),
		)
	})

	When("validating controlled errors", func() {
		It("captures the the chart doesn't exist in the provided path", func() {
			cfg := helm.Config{
				ChartPath: path.Join(chartDir, "not_found"),
			}
			generator := helm.New(cfg)
			_, err := generator.Generate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unable to load chart: stat test_data/not_found: no such file or directory"))
		})
		It("captures the Chart.yaml doesn't exist in the provided path", func() {
			cfg := helm.Config{
				ChartPath: path.Join(chartDir, "missing_chart_yaml"),
			}
			generator := helm.New(cfg)
			_, err := generator.Generate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Chart.yaml file is missing"))
		})
		It("captures the chart has a malformed template", func() {
			cfg := helm.Config{
				ChartPath: path.Join(chartDir, "malformed"),
			}
			generator := helm.New(cfg)
			_, err := generator.Generate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to render the templates for chart k8s_only: template: k8s_only/templates/configmap.yaml:3:23: executing \"k8s_only/templates/configmap.yaml\" at <.Values.foo.bar>: nil pointer evaluating interface {}.bar"))
		})
	})
})

func loadValues(input string, additionalValues map[string]any) map[string]any {
	d, err := os.ReadFile(input)
	Expect(err).NotTo(HaveOccurred())

	var r map[string]any
	Expect(yaml.Unmarshal(d, &r)).NotTo(HaveOccurred())
	maps.Copy(r, additionalValues)
	return r
}
