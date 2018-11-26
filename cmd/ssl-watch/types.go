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
	sha1          string
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

// Service is a struct for a user defined service, which is
// an arbitrary service name, a list of domains with
// optional IP endpoints, and an optional list of named IP sets:
// ---JSON---
// { "serviceName" :
//    "ips" : { "set1" : [ "127.0.0.1", "127.0.0.2", "127.0.0.3" ], "set2": [ "127.0.0.4" ] },
//    "domains" : { "example.com": [], "sample.net": [ "set1", "set2", "127.0.0.5" ] }
// }
// ---JSON---
type Service struct {
	Desc    string              `json:"desc,omitempty"`
	Domains map[string][]string `json:"domains"`
	IPs     map[string][]string `json:"ips,omitempty"`
}

// Services is a wrapper over a map of services with mutex.
// It also includes reverseMap to ease looking up service name
// by domain.
type Services struct {
	db         map[string]Service
	reverseMap map[string]string
	mutex      sync.RWMutex
}

// App is main struct of ssl-watch that,
// after initialization, holds instances of
// Config, Metrics and Domains structures +
// a logger interface.
type App struct {
	config   Config
	services Services
	log      jsonlog.Logger
	metrics  Metrics
}

func (s *Services) Flush() {

	s.mutex.Lock()
	s.db = map[string]Service{}
	s.reverseMap = map[string]string{}
	s.mutex.Unlock()

}

func (s *Services) Update(rawJSON []byte) {

	defer s.mutex.Unlock()
	s.mutex.Lock()
	json.Unmarshal(rawJSON, &s.db)
	for name, service := range s.db {
		for domain := range service.Domains {
			s.reverseMap[domain] = name
		}
	}

}

func (s *Services) ListDomains() []string {

	defer s.mutex.RUnlock()
	s.mutex.RLock()
	domains := []string{}
	for domain := range s.reverseMap {
		domains = append(domains, domain)
	}
	return domains

}

func (s *Services) GetIPs(domain string) []string {

	serviceName, exists := s.GetServiceName(domain)
	ips := []string{}

	if exists {
		s.mutex.RLock()
		service, exists := s.db[serviceName]
		if exists {
			for _, ip := range service.Domains[domain] {
				if !strings.Contains(ip, ".") {
					ips = append(ips, service.IPs[ip]...)
				} else {
					ips = append(ips, ip)
				}
			}
		}
		s.mutex.RUnlock()
	}
	return ips

}

func (s *Services) GetServiceName(domain string) (string, bool) {

	s.mutex.RLock()
	service, exists := s.reverseMap[domain]
	s.mutex.RUnlock()
	return service, exists

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
