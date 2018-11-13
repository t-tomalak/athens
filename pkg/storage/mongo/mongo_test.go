package mongo

import (
	"os"
	"testing"

	"github.com/gomods/athens/pkg/config"
	"github.com/gomods/athens/pkg/storage/compliance"
	"github.com/stretchr/testify/require"
)

func TestBackend(t *testing.T) {
	backend := getStorage(t)
	compliance.RunTests(t, backend, backend.clear)
}

func (m *ModuleStore) clear() error {
	m.s.DB(m.d).C(m.c).DropCollection()

	return m.initDatabase()
}

func BenchmarkBackend(b *testing.B) {
	backend := getStorage(b)
	compliance.RunBenchmarks(b, backend, backend.clear)
}

func getStorage(tb testing.TB) *ModuleStore {
	url := os.Getenv("ATHENS_MONGO_STORAGE_URL")
	if url == "" {
		tb.SkipNow()
	}

	backend, err := NewStorage(&config.MongoConfig{URL: url})
	require.NoError(tb, err)

	return backend
}
