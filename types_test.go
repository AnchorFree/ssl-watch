package main

import (
	"sort"
	"sync"
	"testing"
)

func newServices() *Services {
	return &Services{db: map[string]Service{}, reverseMap: map[string]string{}}
}

// The example from the README.
const readmeConfig = `{
  "mailCerts" :
    {
      "ips" : { "set1" : [ "127.0.0.1", "127.0.0.2", "127.0.0.3" ], "set2": [ "127.0.0.4" ] },
      "domains" : { "example.com:465": [], "sample.net:993": [ "set1", "set2", "127.0.0.5" ] }
    },
  "https" :
    {
      "domains" : { "jack.com": [], "daniels.org:8443": [], "absinth.io": [ "192.168.0.7", "192.168.0.8" ] }
    }
}`

func TestServicesUpdate(t *testing.T) {
	s := newServices()
	s.Update([]byte(readmeConfig))

	domains := s.ListDomains()
	sort.Strings(domains)
	want := []string{"absinth.io", "daniels.org:8443", "example.com:465", "jack.com", "sample.net:993"}
	if len(domains) != len(want) {
		t.Fatalf("ListDomains() = %v, want %v", domains, want)
	}
	for i := range want {
		if domains[i] != want[i] {
			t.Fatalf("ListDomains() = %v, want %v", domains, want)
		}
	}

	for domain, service := range map[string]string{"example.com:465": "mailCerts", "jack.com": "https"} {
		if got, ok := s.GetServiceName(domain); !ok || got != service {
			t.Errorf("GetServiceName(%q) = (%q, %v), want (%q, true)", domain, got, ok, service)
		}
	}
	if _, ok := s.GetServiceName("unknown.com"); ok {
		t.Error("GetServiceName(unknown.com) found a service")
	}
}

func TestServicesUpdateMergesFiles(t *testing.T) {
	s := newServices()
	s.Update([]byte(`{"a": {"domains": {"a.com": []}}}`))
	s.Update([]byte(`{"b": {"domains": {"b.com": []}}}`))
	if got := len(s.ListDomains()); got != 2 {
		t.Errorf("after two files, ListDomains() has %d domains, want 2", got)
	}
}

func TestServicesUpdateIgnoresInvalidJSON(t *testing.T) {
	s := newServices()
	s.Update([]byte(`{"a": {"domains": {"a.com": []}}}`))
	s.Update([]byte(`not json`))
	if got := len(s.ListDomains()); got != 1 {
		t.Errorf("after invalid JSON, ListDomains() has %d domains, want 1", got)
	}
}

func TestServicesGetIPs(t *testing.T) {
	s := newServices()
	s.Update([]byte(readmeConfig))

	tests := map[string][]string{
		// Named sets expand, literal addresses pass through, in config order.
		"sample.net:993":  {"127.0.0.1", "127.0.0.2", "127.0.0.3", "127.0.0.4", "127.0.0.5"},
		"absinth.io":      {"192.168.0.7", "192.168.0.8"},
		"example.com:465": {},
		"unknown.com":     {},
	}
	for domain, want := range tests {
		got := s.GetIPs(domain)
		if len(got) != len(want) {
			t.Errorf("GetIPs(%q) = %v, want %v", domain, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("GetIPs(%q) = %v, want %v", domain, got, want)
				break
			}
		}
	}
}

func TestServicesFlush(t *testing.T) {
	s := newServices()
	s.Update([]byte(readmeConfig))
	s.Flush()
	if got := s.ListDomains(); len(got) != 0 {
		t.Errorf("after Flush, ListDomains() = %v, want none", got)
	}
}

func TestMetrics(t *testing.T) {
	m := Metrics{db: map[string]Endpoints{}}
	m.Set("example.com", Endpoints{"127.0.0.1": {CN: "example.com", alive: true}})

	eps, ok := m.Get("example.com")
	if !ok || eps["127.0.0.1"].CN != "example.com" {
		t.Fatalf("Get(example.com) = (%v, %v)", eps, ok)
	}

	// Get returns a copy, so a caller cannot change what is stored.
	eps["127.0.0.2"] = Endpoint{}
	if stored, _ := m.Get("example.com"); len(stored) != 1 {
		t.Errorf("changing the result of Get changed the stored endpoints: %v", stored)
	}

	m.Flush()
	if _, ok := m.Get("example.com"); ok {
		t.Error("after Flush, Get(example.com) still finds the domain")
	}
}

// The scrape loop, the reload paths and the metrics handler run in separate goroutines.
// Run with -race, this catches access to the maps without the mutex.
func TestConcurrentAccess(t *testing.T) {
	s := newServices()
	m := Metrics{db: map[string]Endpoints{}}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(4)
		go func() { defer wg.Done(); s.Update([]byte(readmeConfig)) }()
		go func() {
			defer wg.Done()
			for _, d := range s.ListDomains() {
				s.GetIPs(d)
				s.GetServiceName(d)
			}
		}()
		go func() { defer wg.Done(); m.Set("example.com", Endpoints{}); m.Flush() }()
		go func() { defer wg.Done(); m.ListDomains(); m.Get("example.com") }()
	}
	wg.Wait()
}
