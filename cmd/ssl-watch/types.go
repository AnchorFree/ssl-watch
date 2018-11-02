package main

import (
	"github.com/anchorfree/golang/pkg/jsonlog"
	"strings"
	"sync"
	"time"
)

type Config struct {
	ConfigDir         string        `default:"/etc/ssl-watch" split_words:"true"`
	ScrapeInterval    time.Duration `default:"60s" split_words:"true"`
	ConnectionTimeout time.Duration `default:"10s" split_words:"true"`
	LookupTimeout     time.Duration `default:"5s" split_words:"true"`
	DebugMode         bool          `default:"false" split_words:"true"`
	Port              string        `default:"9105"`
}

type Endpoint struct {
	CN            string
	AltNamesCount int
	expiry        time.Time
	valid         bool
	alive         bool
}

type Endpoints map[string]Endpoint

type Metrics struct {
	db    map[string]Endpoints
	mutex sync.RWMutex
}

type App struct {
	Domains map[string][]string `json:"domains"`
	config  Config
	log     jsonlog.Logger
	metrics Metrics
}

func (m *Metrics) ListDomains() []string {

	domains := []string{}
	defer m.mutex.RUnlock()
	m.mutex.RLock()
	for domain, _ := range m.db {
		if strings.Contains(domain, ".") {
			domains = append(domains, domain)
		}
	}
	return domains

}

func (m *Metrics) Get(domain string) (Endpoints, bool) {

	defer m.mutex.RUnlock()
	endpoints := Endpoints{}
	m.mutex.RLock()
	_, exists := m.db[domain]
	if exists {
		for k, v := range m.db[domain] {
			endpoints[k] = v
		}
	}
	return endpoints, exists

}

func (m *Metrics) Set(domain string, endpoints Endpoints) {

	defer m.mutex.Unlock()
	m.mutex.Lock()
	m.db[domain] = endpoints

}
