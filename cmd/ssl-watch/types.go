package main

import (
	"encoding/json"
	"github.com/anchorfree/golang/pkg/jsonlog"
	"strings"
	"sync"
	"time"
)

// Config holds processed configuration from environment variables
type Config struct {
	ConfigDir         string        `default:"/etc/ssl-watch" split_words:"true"`
	ScrapeInterval    time.Duration `default:"60s" split_words:"true"`
	ConnectionTimeout time.Duration `default:"10s" split_words:"true"`
	LookupTimeout     time.Duration `default:"5s" split_words:"true"`
	DebugMode         bool          `default:"false" split_words:"true"`
	Port              string        `default:"9105"`
}

// Endpoint is a struct for holding info about a single domain endpoint,
// i.e. IP address. If we can't connect to this endpoint, we set alive
// to false.
type Endpoint struct {
	CN            string
	AltNamesCount int
	expiry        time.Time
	valid         bool
	alive         bool
}

// Endpoints is a map of domains to endpoints.
type Endpoints map[string]Endpoint

// Metrics is basically just a wrapper around Endpoints + mutex.
type Metrics struct {
	db    map[string]Endpoints
	mutex sync.RWMutex
}

// Domains is a struct to hold parsed information from
// config files.
type Domains struct {
	db    map[string][]string
	mutex sync.RWMutex
}

// App is main struct of ssl-watch that,
// after initialization, holds instances of
// Config, Metrics and Domains structures +
// a logger interface.
type App struct {
	domains Domains
	config  Config
	log     jsonlog.Logger
	metrics Metrics
}

// Flush flushes all the values from Domains map.
func (d *Domains) Flush() {

	d.mutex.Lock()
	d.db = map[string][]string{}
	d.mutex.Unlock()

}

// Update takes a []byte of JSON and unmarshals
// it into Domains map.
func (d *Domains) Update(rawJSON []byte) {

	d.mutex.Lock()
	json.Unmarshal([]byte(rawJSON), &d.db)
	d.mutex.Unlock()

}

// List returns a string slice of all
// domain names in the Domains map.
func (d *Domains) List() []string {

	defer d.mutex.RUnlock()
	domains := []string{}
	d.mutex.RLock()
	for domain := range d.db {
		if strings.Contains(domain, ".") {
			domains = append(domains, domain)
		}
	}
	return domains
}

// GetIPs returns a string slice of IP addresses
// for a given domain from the Domains map.
func (d *Domains) GetIPs(domain string) []string {

	defer d.mutex.RUnlock()
	ips := []string{}
	d.mutex.RLock()

	addrSet, ok := d.db[domain]
	if ok && len(addrSet) > 0 {
		if !strings.Contains(addrSet[0], ".") {
			addrSet, ok = d.db[addrSet[0]]
		}
		for i := range addrSet {
			ips = append(ips, addrSet[i])
		}
	}
	return ips

}

func (m *Metrics) ListDomains() []string {

	defer m.mutex.RUnlock()
	domains := []string{}
	m.mutex.RLock()
	for domain := range m.db {
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
