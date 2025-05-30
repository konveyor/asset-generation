package helm

import (
	"fmt"
	"maps"
	"strings"

	"github.com/konveyor/asset-generation/pkg/providers"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

const (
	konveyorDirectoryName = "files/konveyor"
)

type Config struct {
	ChartPath                 string
	Values                    map[string]any
	SkipRenderK8SManifests    bool
	SkipRenderNonK8SManifests bool
}

type helmProvider struct {
	cfg Config
}

func New(cfg Config) providers.Generator {
	return &helmProvider{cfg: cfg}
}

func (p *helmProvider) Generate() (map[string]string, error) {
	chart, err := p.loadChart()
	if err != nil {
		return nil, err
	}
	chart.Values = p.cfg.Values
	rendered := make(map[string]string)
	if !p.cfg.SkipRenderK8SManifests {
		rendered, err = generateK8sTemplates(*chart)
		if err != nil {
			return nil, err
		}
	}
	if !p.cfg.SkipRenderNonK8SManifests {
		r, err := generateNonK8sTemplates(*chart)
		if err != nil {
			return nil, err
		}
		maps.Copy(rendered, r)
	}
	return rendered, nil
}

func (p *helmProvider) loadChart() (*chart.Chart, error) {
	l, err := loader.Loader(p.cfg.ChartPath)
	if err != nil {
		return nil, fmt.Errorf("unable to load chart: %s", err)
	}
	return l.Load()
}

func generateK8sTemplates(chart chart.Chart) (map[string]string, error) {
	return generateTemplates(chart)
}

func generateNonK8sTemplates(chart chart.Chart) (map[string]string, error) {
	chart.Templates = filterTemplatesByPath(konveyorDirectoryName, chart.Files)
	return generateTemplates(chart)
}

func generateTemplates(chart chart.Chart) (map[string]string, error) {
	e := engine.Engine{}
	options := chartutil.ReleaseOptions{
		Name:      chart.Name(),
		Namespace: "",
		Revision:  1,
		IsInstall: false,
		IsUpgrade: false,
	}
	valuesToRender, err := chartutil.ToRenderValues(&chart, chart.Values, options, chartutil.DefaultCapabilities.Copy())
	if err != nil {
		return nil, fmt.Errorf("failed to render the values for chart %s: %s", chart.Name(), err)
	}
	chart.Values = valuesToRender
	rendered, err := e.Render(&chart, valuesToRender)
	if err != nil {
		return nil, fmt.Errorf("failed to render the templates for chart %s: %s", chart.Name(), err)
	}
	return rendered, nil

}

func filterTemplatesByPath(pathPrefix string, files []*chart.File) []*chart.File {
	ret := []*chart.File{}
	for _, f := range files {
		if strings.HasPrefix(f.Name, pathPrefix) {
			ret = append(ret, f)
		}
	}
	return ret
}
