package main

import (
	"encoding/json"
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

type Domains struct {
	db    map[string][]string
	mutex sync.RWMutex
}

type App struct {
	domains Domains
	config  Config
	log     jsonlog.Logger
	metrics Metrics
}

func (d *Domains) Flush() {

	d.mutex.Lock()
	d.db = map[string][]string{}
	d.mutex.Unlock()

}

func (d *Domains) Update(rawJSON []byte) {

	d.mutex.Lock()
	json.Unmarshal([]byte(rawJSON), &d.db)
	d.mutex.Unlock()

}

func (d *Domains) List() []string {

	defer d.mutex.RUnlock()
	domains := []string{}
	d.mutex.RLock()
	for domain, _ := range d.db {
		if strings.Contains(domain, ".") {
			domains = append(domains, domain)
		}
	}
	return domains
}

func (d *Domains) GetIPs(domain string) []string {

	defer d.mutex.RUnlock()
	ips := []string{}
	d.mutex.RLock()

	addrSet, ok := d.db[domain]
	if ok && len(addrSet) > 0 {
		if !strings.Contains(addrSet[0], ".") {
			addrSet, ok = d.db[addrSet[0]]
		}
		for _, ip := range addrSet {
			ips = append(ips, ip)
		}
	}
	return ips

}

func (m *Metrics) ListDomains() []string {

	defer m.mutex.RUnlock()
	domains := []string{}
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

	m.mutex.Lock()
	m.db[domain] = endpoints
	m.mutex.Unlock()

}

func (m *Metrics) Flush() {

	m.mutex.Lock()
	m.db = map[string]Endpoints{}
	m.mutex.Unlock()

}
