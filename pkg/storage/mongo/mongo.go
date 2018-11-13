package mongo

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/globalsign/mgo"
	"github.com/gomods/athens/pkg/config"
	"github.com/gomods/athens/pkg/errors"
)

// ModuleStore represents a mongo backed storage backend.
type ModuleStore struct {
	s        *mgo.Session
	d        string // database
	c        string // collection
	url      string
	certPath string
	insecure bool // Only to be used for development instances
	timeout  time.Duration
}

// NewStorage returns a connected Mongo backed storage
// that satisfies the Backend interface.
func NewStorage(conf *config.MongoConfig) (*ModuleStore, error) {
	const op errors.Op = "mongo.NewStorage"
	if conf == nil {
		return nil, errors.E(op, "No Mongo Configuration provided")
	}
	ms := &ModuleStore{url: conf.URL, certPath: conf.CertPath, timeout: conf.TimeoutDuration()}

	err := ms.connect()
	if err != nil {
		return nil, errors.E(op, err)
	}

	return ms, nil
}

func (m *ModuleStore) connect() error {
	const op errors.Op = "mongo.connect"

	var err error
	m.s, err = m.newSession(m.timeout, m.insecure)
	if err != nil {
		return errors.E(op, err)
	}

	return m.initDatabase()
}

func (m *ModuleStore) initDatabase() error {
	// TODO: database and collection as env vars, or params to New()? together with user/mongo
	m.d = "athens"
	m.c = "modules"

	index := mgo.Index{
		Key:        []string{"base_url", "module", "version"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	c := m.s.DB(m.d).C(m.c)
	return c.EnsureIndex(index)
}

func (m *ModuleStore) newSession(timeout time.Duration, insecure bool) (*mgo.Session, error) {
	tlsConfig := &tls.Config{}

	dialInfo, err := mgo.ParseURL(m.url)
	if err != nil {
		return nil, err
	}

	dialInfo.Timeout = timeout

	if m.certPath != "" {
		// Sets only when the env var is setup in config.dev.toml
		tlsConfig.InsecureSkipVerify = insecure
		var roots *x509.CertPool
		// See if there is a system cert pool
		roots, err = x509.SystemCertPool()
		if err != nil {
			// If there is no system cert pool, create a new one
			roots = x509.NewCertPool()
		}

		cert, err := ioutil.ReadFile(m.certPath)
		if err != nil {
			return nil, err
		}

		if ok := roots.AppendCertsFromPEM(cert); !ok {
			return nil, fmt.Errorf("failed to parse certificate from: %s", m.certPath)
		}

		tlsConfig.ClientCAs = roots

		dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
			return tls.Dial("tcp", addr.String(), tlsConfig)
		}
	}

	return mgo.DialWithInfo(dialInfo)
}

func (m *ModuleStore) gridFileName(mod, ver string) string {
	return strings.Replace(mod, "/", "_", -1) + "_" + ver + ".zip"
}
