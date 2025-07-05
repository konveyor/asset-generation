package cloud_foundry

import (
	"encoding/json"
	"fmt"
	"testing"

	"net/http"
	"strconv"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/cloudfoundry/go-cfclient/v3/testutil"
	"github.com/konveyor/asset-generation/internal/models"
	. "github.com/onsi/gomega"
)

const (
	pagingQueryString = "page=1&per_page=50"
)

func toJSON(obj any) string {
	o, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return string(o)
}

type mockApplication struct {
	app        models.AppManifest
	resMap     map[string]any
	g          *testutil.ObjectJSONGenerator
	mockRoutes []testutil.MockRoute
}

func (m *mockApplication) application() *testutil.JSONResource {

	if app, ok := m.resMap["app"]; ok {
		return app.(*testutil.JSONResource)
	}
	appManifest := emptyResource()
	ar := resource.App{}
	ar.Name = m.app.Name
	ar.GUID = appManifest.GUID
	appManifest.Name = m.app.Name
	var appType string
	switch {
	case m.app.Docker != nil && len(m.app.Docker.Image) > 0:
		appType = "docker"
	case len(m.app.Buildpacks) > 0:
		appType = "buildpack"
	}
	ar.Lifecycle = resource.Lifecycle{
		Type:          appType,
		BuildpackData: resource.BuildpackLifecycle{Stack: m.app.Stack, Buildpacks: m.app.Buildpacks},
	}
	ar.Relationships.Space.Data = &resource.Relationship{GUID: m.space().GUID}
	ar.Metadata = (*resource.Metadata)(m.app.Metadata)
	appManifest.JSON = toJSON(ar)
	m.resMap["app"] = appManifest
	return appManifest
}

func (m *mockApplication) droplet() *testutil.JSONResource {
	if d, ok := m.resMap["droplet"]; ok {
		return d.(*testutil.JSONResource)
	}
	d := emptyResource()
	if m.app.Docker == nil || (m.app.Docker != nil && len(m.app.Docker.Image) == 0) {
		m.resMap["droplet"] = resource.Droplet{}
		return d
	}
	droplet := resource.Droplet{
		Image: &m.app.Docker.Image,
	}
	d.JSON = toJSON(droplet)
	m.resMap["droplet"] = d
	return d
}

func (m *mockApplication) services() map[string]json.RawMessage {
	svcs := map[string]json.RawMessage{}
	if svc, ok := m.resMap["services"]; ok {
		return svc.(map[string]json.RawMessage)
	}
	if m.app.Services == nil {
		return svcs
	}
	vcap := map[string]appVCAPServiceAttributes{}
	for _, svc := range *m.app.Services {
		var creds json.RawMessage
		if svc.Parameters != nil {
			creds = json.RawMessage(toJSON(svc.Parameters))
		}
		vcap[svc.Name] = appVCAPServiceAttributes{Name: svc.BindingName, Credentials: creds}
	}
	svcs[vcapServices] = json.RawMessage(toJSON(vcap))
	m.resMap["services"] = svcs
	return svcs
}

func (m *mockApplication) env() *testutil.JSONResource {

	if env, ok := m.resMap["env"]; ok {
		return env.(*testutil.JSONResource)
	}
	env := m.g.AppEnvironment()
	er := resource.AppEnvironment{}
	b := toJSON(m.app.Env)
	Expect(json.Unmarshal([]byte(b), &er.EnvVars)).NotTo(HaveOccurred())
	er.SystemEnvVars = m.services()
	env.JSON = toJSON(er)

	m.resMap["env"] = env
	return env
}

func ptrTo[T any](obj T) *T {
	return &obj
}
func (m *mockApplication) parseProcesses() map[string]*testutil.JSONResource {
	mp := map[string]*testutil.JSONResource{}
	m.resMap["processes"] = mp
	inline := models.AppManifestProcess{}
	b, err := json.Marshal(m.app)
	Expect(err).NotTo(HaveOccurred())
	Expect(json.Unmarshal(b, &inline)).NotTo(HaveOccurred())
	procs := []models.AppManifestProcess{}
	if m.app.Processes != nil {
		procs = []models.AppManifestProcess(*m.app.Processes)
	}
	if len(inline.Type) > 0 {
		procs = append(procs, inline)
	}
	if len(procs) == 0 {
		return mp
	}

	for _, p := range procs {
		t := m.g.Process()
		mem, err := strconv.Atoi(p.Memory)
		Expect(err).NotTo(HaveOccurred())
		var disk int
		if len(p.DiskQuota) > 0 {
			disk, err = strconv.Atoi(p.DiskQuota)
			Expect(err).NotTo(HaveOccurred())
		}
		lograte, err := strconv.Atoi(p.LogRateLimitPerSecond)
		Expect(err).NotTo(HaveOccurred())
		resProc := resource.Process{
			Type:                         string(p.Type),
			Instances:                    int(*p.Instances),
			Command:                      &p.Command,
			MemoryInMB:                   mem,
			DiskInMB:                     disk,
			LogRateLimitInBytesPerSecond: lograte,
			HealthCheck: resource.ProcessHealthCheck{
				Type: string(p.HealthCheckType),
				Data: resource.ProcessHealthCheckData{
					Timeout:           ptrTo(int(p.Timeout)),
					InvocationTimeout: ptrTo(int(p.HealthCheckInvocationTimeout)),
					Interval:          ptrTo(int(p.HealthCheckInterval)),
					Endpoint:          &p.HealthCheckHTTPEndpoint,
				},
			},
			ReadinessCheck: resource.ProcessReadinessCheck{
				Type: string(p.ReadinessHealthCheckType),
				Data: resource.ProcessReadinessCheckData{
					InvocationTimeout: ptrTo(int(p.ReadinessHealthInvocationTimeout)),
					Interval:          ptrTo(int(p.ReadinessHealthCheckInterval)),
					Endpoint:          &p.ReadinessHealthCheckHttpEndpoint,
				},
			},
			Resource: resource.Resource{
				GUID: t.GUID,
			},
		}
		t.JSON = toJSON(resProc)
		mp[t.GUID] = t
		// Add the process individual route so that it can be fetched as well
		m.mockRoutes = append(m.mockRoutes, m.generateMockRoute("/v3/processes/"+t.GUID, m.g.Single(t.JSON), ""))
	}
	m.resMap["processes"] = mp
	return mp
}
func (m *mockApplication) processes() []string {

	p, ok := m.resMap["processes"]
	if ok {
		return p.([]string)
	}
	p = m.parseProcesses()
	pp := p.(map[string]*testutil.JSONResource)
	plist := []string{}
	for _, proc := range pp {
		resProc := resource.Process{}
		Expect(json.Unmarshal([]byte(proc.JSON), &resProc)).NotTo(HaveOccurred())
		plist = append(plist, toJSON(resProc))
	}

	// Create a temporal JSONResource whose JSON field contains the list of processes.
	m.resMap["processes"] = plist
	return plist
}

func (m *mockApplication) space() *testutil.JSONResource {
	if _, ok := m.resMap["space"]; !ok {
		m.resMap["space"] = m.g.Space()
	}
	return m.resMap["space"].(*testutil.JSONResource)
}

func (m *mockApplication) route(route models.AppManifestRoute) resource.Route {

	testRoute := m.g.Route()
	rr := resource.Route{
		URL:      string(route.Route),
		Protocol: "http1", // we don't capture it, so static to http1
		Resource: resource.Resource{GUID: testRoute.GUID},
	}
	testRoute.JSON = toJSON(rr)
	// Add the destination
	testDestination := m.destinationFor()
	m.mockRoutes = append(m.mockRoutes, m.generateMockRoute("/v3/routes/"+testRoute.GUID+"/destinations", m.g.Single(testDestination.JSON), ""))
	return rr
}

func (m *mockApplication) routes() []string {
	if v, ok := m.resMap["routes"]; ok {
		return v.([]string)
	}
	rList := []string{}
	m.resMap["routes"] = rList
	if m.app.NoRoute || m.app.Routes == nil {
		return rList
	}
	for _, v := range *m.app.Routes {
		rList = append(rList, toJSON(m.route(v)))
	}
	m.resMap["routes"] = rList
	return rList
}

func (m *mockApplication) destinationFor() testutil.JSONResource {
	d := testutil.JSONResource{
		GUID: testutil.RandomGUID(),
		Name: testutil.RandomName(),
	}
	ds := resource.RouteDestinations{}
	for _, v := range *m.app.Routes {
		ds.Destinations = append(ds.Destinations, &resource.RouteDestination{
			Protocol: (*string)(&v.Protocol),
			App:      resource.RouteDestinationApp{GUID: &m.application().GUID},
		})
	}
	d.JSON = toJSON(ds)
	return d
}

func emptyResource() *testutil.JSONResource {
	return &testutil.JSONResource{
		GUID: testutil.RandomGUID(),
		Name: testutil.RandomName(),
		JSON: "{}",
	}
}
func (m *mockApplication) sidecars() []string {
	if v, ok := m.resMap["sidecars"]; ok {
		return v.([]string)
	}
	if m.app.Sidecars == nil {
		return nil
	}
	sidecars := []string{}
	for _, v := range *m.app.Sidecars {
		pt := toJSON(v.ProcessTypes)
		sptypes := []string{}
		Expect(json.Unmarshal([]byte(pt), &sptypes)).ToNot(HaveOccurred())
		var mem int
		var err error
		if len(v.Memory) > 0 {
			mem, err = strconv.Atoi(v.Memory)
			Expect(err).NotTo(HaveOccurred())
		}
		sc := resource.Sidecar{
			Name:         v.Name,
			Command:      v.Command,
			ProcessTypes: sptypes,
			MemoryInMB:   mem,
			Origin:       "user",
		}
		sidecars = append(sidecars, toJSON(sc))
	}
	m.resMap["sidecars"] = sidecars
	return sidecars

}

func newMockApplication(app models.AppManifest, t *testing.T) (mockApplication, string) {
	m := mockApplication{
		g:      testutil.NewObjectJSONGenerator(),
		app:    app,
		resMap: map[string]any{},
	}
	m.mockRoutes = m.setupMockRoutes()
	return m, testutil.SetupMultiple(m.mockRoutes, t)
}

const (
	v3apps = "/v3/apps/"
)

func (m *mockApplication) generateMockRoute(endpoint string, output []string, query string) testutil.MockRoute {
	return testutil.MockRoute{
		Method:      http.MethodGet,
		Endpoint:    endpoint,
		Output:      output,
		Status:      http.StatusOK,
		QueryString: query,
	}
}

func (m *mockApplication) setupMockRoutes() []testutil.MockRoute {
	var routes []testutil.MockRoute

	routes = append(routes,
		m.generateMockRoute("/v3/apps", m.g.Paged([]string{m.application().JSON}), "names="+m.application().Name+"&"+pagingQueryString+"&space_guids="+m.space().GUID),
		m.generateMockRoute("/v3/spaces", m.g.Paged([]string{m.space().JSON}), "names="+m.space().Name+"&"+pagingQueryString),
		m.generateMockRoute(fmt.Sprintf(v3apps+m.application().GUID), m.g.Single(m.application().JSON), ""),
		m.generateMockRoute(fmt.Sprintf(v3apps+m.application().GUID+"/env"), m.g.Single(m.env().JSON), ""),
		m.generateMockRoute(fmt.Sprintf(v3apps+m.application().GUID+"/processes"), m.g.Paged(m.processes()), pagingQueryString),
		m.generateMockRoute(fmt.Sprintf(v3apps+m.application().GUID+"/routes"), m.g.Paged(m.routes()), ""),
		m.generateMockRoute(fmt.Sprintf(v3apps+m.application().GUID+"/sidecars"), m.g.Paged(m.sidecars()), ""),
		m.generateMockRoute(fmt.Sprintf(v3apps+m.application().GUID+"/droplets/current"), m.g.Single(m.droplet().JSON), ""),
	)
	return append(m.mockRoutes, routes...)
}
