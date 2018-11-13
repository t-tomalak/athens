package download

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gomods/athens/pkg/config"
	"github.com/gomods/athens/pkg/module"
	"github.com/gomods/athens/pkg/stash"
	"github.com/gomods/athens/pkg/storage"
	"github.com/gomods/athens/pkg/storage/mem"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

var (
	testConfigPath = filepath.Join("..", "..", "config.dev.toml")
)

func getDP(t *testing.T) Protocol {
	t.Helper()
	conf, err := config.GetConf(testConfigPath)
	if err != nil {
		t.Fatalf("Unable to parse config file: %s", err.Error())
	}
	goBin := conf.GoBinary
	fs := afero.NewOsFs()
	mf, err := module.NewGoGetFetcher(goBin, fs)
	if err != nil {
		t.Fatal(err)
	}
	s, err := mem.NewStorage()
	if err != nil {
		t.Fatal(err)
	}
	st := stash.New(mf, s)
	return New(&Opts{s, st, NewVCSLister(goBin, fs)})
}

type listTest struct {
	name string
	path string
	tags []string
}

var listTests = []listTest{
	{
		name: "happy tags",
		path: "github.com/athens-artifacts/happy-path",
		tags: []string{"v0.0.1", "v0.0.2", "v0.0.3"},
	},
	{
		name: "no tags",
		path: "github.com/athens-artifacts/no-tags",
		tags: []string{},
	},
}

func TestList(t *testing.T) {
	dp := getDP(t)
	ctx := context.Background()

	for _, tc := range listTests {
		t.Run(tc.name, func(t *testing.T) {
			versions, err := dp.List(ctx, tc.path)
			require.NoError(t, err)
			require.EqualValues(t, tc.tags, versions)
		})
	}
}

func TestConcurrentLists(t *testing.T) {
	dp := getDP(t)
	ctx := context.Background()

	pkg := "github.com/athens-artifacts/samplelib"
	var pkgErr error

	subPkg := "github.com/athens-artifacts/samplelib/types"
	var subPkgErr error

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		_, pkgErr = dp.List(ctx, pkg)
		wg.Done()
	}()
	go func() {
		_, subPkgErr = dp.List(ctx, subPkg)
		wg.Done()
	}()
	wg.Wait()

	if pkgErr != nil {
		t.Fatalf("expected version listing of %v to succeed but got %v", pkg, pkgErr)
	}

	if subPkgErr == nil {
		t.Fatalf("expected version listing of %v to fail because it's a subdirectory", subPkg)
	}
}

type latestTest struct {
	name string
	path string
	info *storage.RevInfo
	err  bool
}

var latestTests = []latestTest{
	{
		name: "happy path",
		path: "github.com/athens-artifacts/no-tags",
		info: &storage.RevInfo{
			Version: "v0.0.0-20180803171426-1a540c5d67ab",
			Time:    time.Date(2018, 8, 3, 17, 14, 26, 0, time.UTC),
		},
	},
	{
		name: "tagged latest",
		path: "github.com/athens-artifacts/happy-path",
		info: &storage.RevInfo{
			Version: "v0.0.3",
			Time:    time.Date(2018, 8, 3, 17, 16, 00, 0, time.UTC),
		},
	},
}

func TestLatest(t *testing.T) {
	dp := getDP(t)
	ctx := context.Background()

	for _, tc := range latestTests {
		t.Run(tc.name, func(t *testing.T) {
			info, err := dp.Latest(ctx, tc.path)
			if !tc.err && err != nil {
				t.Fatal(err)
			} else if tc.err && err == nil {
				t.Fatalf("expected %v error but got nil", tc.err)
			}

			require.EqualValues(t, tc.info, info)
		})
	}
}

type infoTest struct {
	name    string
	path    string
	version string
	info    *storage.RevInfo
}

var infoTests = []infoTest{
	{
		name:    "happy path",
		path:    "github.com/athens-artifacts/happy-path",
		version: "v0.0.2",
		info: &storage.RevInfo{
			Version: "v0.0.2",
			Time:    time.Date(2018, 8, 3, 3, 45, 19, 0, time.UTC),
		},
	},
	{
		name:    "pseudo version",
		path:    "github.com/athens-artifacts/no-tags",
		version: "v0.0.0-20180803035119-e4e0177efdb5",
		info: &storage.RevInfo{
			Version: "v0.0.0-20180803035119-e4e0177efdb5",
			Time:    time.Date(2018, 8, 3, 3, 51, 19, 0, time.UTC),
		},
	},
}

func TestInfo(t *testing.T) {
	dp := getDP(t)
	ctx := context.Background()

	for _, tc := range infoTests {
		t.Run(tc.name, func(t *testing.T) {
			bts, err := dp.Info(ctx, tc.path, tc.version)
			require.NoError(t, err)

			var info storage.RevInfo
			dec := json.NewDecoder(bytes.NewReader(bts))
			dec.DisallowUnknownFields()
			err = dec.Decode(&info)
			require.NoError(t, err)

			require.EqualValues(t, tc.info, &info)
		})
	}
}

type modTest struct {
	name    string
	path    string
	version string
	err     bool
}

var modTests = []modTest{
	{
		name:    "no mod file",
		path:    "github.com/athens-artifacts/no-tags",
		version: "v0.0.0-20180803035119-e4e0177efdb5",
	},
	{
		name:    "upstream mod file",
		path:    "github.com/athens-artifacts/happy-path",
		version: "v0.0.3",
	},
	{
		name:    "incorrect github repo",
		path:    "github.com/athens-artifacts/not-exists",
		version: "v1.0.0",
		err:     true,
	},
}

func rmNewLine(input string) string {
	re := regexp.MustCompile(`\r?\n`)
	return re.ReplaceAllString(input, "")
}

func TestGoMod(t *testing.T) {
	dp := getDP(t)
	ctx := context.Background()

	for _, tc := range modTests {
		t.Run(tc.name, func(t *testing.T) {
			mod, err := dp.GoMod(ctx, tc.path, tc.version)
			require.Equal(t, tc.err, err != nil, err)

			if tc.err {
				t.Skip()
			}
			expected := rmNewLine(string(getGoldenFile(t, tc.name)))
			res := rmNewLine(string(mod))
			require.Equal(t, expected, res)
		})
	}
}

func getGoldenFile(t *testing.T, name string) []byte {
	t.Helper()
	file := filepath.Join("test_data", strings.Replace(name, " ", "_", -1)+".golden")
	bts, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	return bts
}

type testMod struct {
	mod, ver string
}

var mods = []testMod{
	{"github.com/athens-artifacts/no-tags", "v0.0.2"},
	{"github.com/athens-artifacts/happy-path", "v0.0.0-20180803035119-e4e0177efdb5"},
	{"github.com/athens-artifacts/samplelib", "v1.0.0"},
}

func TestDownloadProtocol(t *testing.T) {
	s, err := mem.NewStorage()
	if err != nil {
		t.Fatal(err)
	}
	mp := &mockFetcher{}
	st := stash.New(mp, s)
	dp := New(&Opts{s, st, nil})
	ctx := context.Background()

	var eg errgroup.Group
	for i := 0; i < len(mods); i++ {
		m := mods[i]
		eg.Go(func() error {
			_, err := dp.GoMod(ctx, m.mod, m.ver)
			return err
		})
	}

	err = eg.Wait()
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range mods {
		bts, err := dp.GoMod(ctx, m.mod, m.ver)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(bts, []byte(m.mod+"@"+m.ver)) {
			t.Fatalf("unexpected gomod content: %s", bts)
		}
	}
}

type mockFetcher struct{}

func (m *mockFetcher) Fetch(ctx context.Context, mod, ver string) (*storage.Version, error) {
	bts := []byte(mod + "@" + ver)
	return &storage.Version{
		Mod:  bts,
		Info: bts,
		Zip:  ioutil.NopCloser(bytes.NewReader(bts)),
	}, nil
}
