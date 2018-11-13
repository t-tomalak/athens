package middleware

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gobuffalo/buffalo"
	"github.com/gomods/athens/pkg/config"
	"github.com/gomods/athens/pkg/module"
	"github.com/markbates/willie"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Avoid import cycle.
const (
	pathList        = "/{module:.+}/@v/list"
	pathVersionInfo = "/{module:.+}/@v/{version}.info"
)

var (
	testConfigFile = filepath.Join("..", "..", "config.dev.toml")
)

func middlewareFilterApp(filterFile, registryEndpoint string) (*buffalo.App, error) {
	h := func(c buffalo.Context) error {
		return c.Render(200, nil)
	}

	a := buffalo.New(buffalo.Options{})
	mf, err := newTestFilter(filterFile)
	if err != nil {
		return nil, err
	}
	a.Use(NewFilterMiddleware(mf, registryEndpoint))

	a.GET(pathList, h)
	a.GET(pathVersionInfo, h)
	return a, nil
}

func newTestFilter(filterFile string) (*module.Filter, error) {
	f, err := module.NewFilter(filterFile)
	if err != nil {
		return nil, err
	}
	f.AddRule("github.com/gomods/athens/", module.Direct)
	f.AddRule("github.com/athens-artifacts/no-tags", module.Exclude)
	f.AddRule("github.com/athens-artifacts", module.Include)
	return f, nil
}

func Test_FilterMiddleware(t *testing.T) {
	r := require.New(t)

	filter, err := ioutil.TempFile(os.TempDir(), "filter-")
	if err != nil {
		t.FailNow()
	}
	defer os.Remove(filter.Name())

	conf, err := config.GetConf(testConfigFile)
	if err != nil {
		t.Fatalf("Unable to parse config file: %s", err.Error())
	}

	// Test with a filter file not existing
	app, err := middlewareFilterApp("nofsfile", conf.GlobalEndpoint)
	r.Nil(app, "app should be nil when a file not exisiting")
	r.Error(err, "Expected error when a file not existing on the filesystem is given")

	app, err = middlewareFilterApp(filter.Name(), conf.GlobalEndpoint)
	r.NoError(err, "app should be succesfully created in the test")
	w := willie.New(app)

	// Public, expects to be redirected to the global registry endpoint, with and without a trailing slash
	paths := []string{"/github.com/gomods/athens/@v/list/", "/github.com/gomods/athens/@v/list"}
	for _, path := range paths {
		res := w.Request(path).Get()
		r.Equal(303, res.Code)
		r.Equal(conf.GlobalEndpoint+"/github.com/gomods/athens/@v/list/", res.HeaderMap.Get("Location"))
	}

	// Excluded, expects a 403
	res := w.Request("/github.com/athens-artifacts/no-tags/@v/list").Get()
	r.Equal(403, res.Code)

	// Private, the proxy is working and returns a 200
	res = w.Request("/github.com/athens-artifacts/happy-path/@v/list").Get()
	r.Equal(200, res.Code)
}

func hookFilterApp(hook string) *buffalo.App {
	h := func(c buffalo.Context) error {
		return c.Render(200, nil)
	}

	a := buffalo.New(buffalo.Options{})
	a.Use(NewValidationMiddleware(hook))

	a.GET(pathList, h)
	a.GET(pathVersionInfo, h)
	return a
}

type hookMock struct {
	invoked bool
	params  validationParams
	resCode int
}

func (m *hookMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.invoked = true
	w.WriteHeader(m.resCode)
	decoder := json.NewDecoder(r.Body)
	decoder.Decode(&m.params)
}

type HookTestsSuite struct {
	suite.Suite
	mock   hookMock
	server *httptest.Server
	w      *willie.Willie
}

func (suite *HookTestsSuite) SetupSuite() {
	suite.server = httptest.NewServer(&suite.mock)
	suite.w = willie.New(hookFilterApp(suite.server.URL))
}

func (suite *HookTestsSuite) SetupTest() {
	suite.mock.invoked = false
	suite.mock.resCode = 0
}

func (suite *HookTestsSuite) TearDownSuite() {
	suite.server.Close()
}

func TestHookTestSuite(t *testing.T) {
	suite.Run(t, new(HookTestsSuite))
}

func (suite *HookTestsSuite) TestHookOnList() {
	r := suite.Require()
	// list path, hook should not be hit
	suite.w.Request("/github.com/gomods/athens/@v/list").Get()
	r.False(suite.mock.invoked)
}

func (suite *HookTestsSuite) TestHookPass() {
	r := suite.Require()
	// hit and pass
	suite.mock.resCode = http.StatusOK
	res := suite.w.Request("/github.com/athens-artifacts/happy-path/@v/v1.0.0.info").Get()
	r.True(suite.mock.invoked)
	r.Equal(http.StatusOK, res.Code)
	r.Equal("github.com/athens-artifacts/happy-path", suite.mock.params.Module)
	r.Equal("v1.0.0", suite.mock.params.Version)
}

func (suite *HookTestsSuite) TestHookBlocks() {
	r := suite.Require()

	// hit but hook blocks
	suite.mock.resCode = http.StatusForbidden
	res := suite.w.Request("/github.com/athens-artifacts/happy-path/@v/v1.0.0.info").Get()
	r.True(suite.mock.invoked)
	r.Equal(http.StatusForbidden, res.Code)
}

func (suite *HookTestsSuite) TestHookUnexpectedError() {
	r := suite.Require()

	// hit but unexpected error
	suite.mock.resCode = http.StatusGone
	res := suite.w.Request("/github.com/athens-artifacts/happy-path/@v/v1.0.0.info").Get()
	r.True(suite.mock.invoked)
	r.Equal(http.StatusInternalServerError, res.Code)
}
