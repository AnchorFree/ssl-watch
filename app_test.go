package main

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestApp() *App {
	return &App{
		config: Config{
			ConfigFileSuffix:  ".conf",
			ConnectionTimeout: 2 * time.Second,
			LookupTimeout:     2 * time.Second,
		},
		log:      *zap.NewNop(),
		metrics:  Metrics{mutex: sync.RWMutex{}, db: map[string]Endpoints{}},
		services: Services{mutex: sync.RWMutex{}, db: map[string]Service{}, reverseMap: map[string]string{}},
	}
}

// newTLSServer starts a TLS server on 127.0.0.1. The httptest certificate is valid for
// example.com and 127.0.0.1. If maxVersion is not 0, the server speaks no TLS version above it.
func newTLSServer(t *testing.T, maxVersion uint16) (port string, cert []byte) {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.NotFoundHandler())
	srv.TLS = &tls.Config{MaxVersion: maxVersion}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	_, port, _ = net.SplitHostPort(srv.Listener.Addr().String())
	return port, srv.Certificate().Raw
}

func TestProcessDomainValidCertificate(t *testing.T) {
	port, raw := newTLSServer(t, 0)
	app := newTestApp()

	eps := app.ProcessDomain("example.com:"+port, []net.IP{net.ParseIP("127.0.0.1")})
	ep, ok := eps["127.0.0.1"]
	if !ok {
		t.Fatalf("ProcessDomain returned no endpoint for 127.0.0.1: %v", eps)
	}
	if !ep.alive || !ep.valid {
		t.Errorf("endpoint alive=%v valid=%v, want both true", ep.alive, ep.valid)
	}
	if ep.expiry.IsZero() || ep.AltNamesCount == 0 {
		t.Errorf("certificate details missing: %+v", ep)
	}
	sum := sha256.Sum256(raw)
	if want := hex.EncodeToString(sum[:]); ep.sha256 != want {
		t.Errorf("sha256 = %q, want %q", ep.sha256, want)
	}
}

func TestProcessDomainHostnameMismatch(t *testing.T) {
	port, _ := newTLSServer(t, 0)
	app := newTestApp()

	// The connection succeeds, because ssl-watch skips verification so it can report on
	// invalid and expired certificates, but the certificate does not cover the domain.
	ep := app.ProcessDomain("other.org:"+port, []net.IP{net.ParseIP("127.0.0.1")})["127.0.0.1"]
	if !ep.alive || ep.valid {
		t.Errorf("endpoint alive=%v valid=%v, want alive and not valid", ep.alive, ep.valid)
	}
}

func TestProcessDomainDeadEndpoint(t *testing.T) {
	// Take a free port, then close it, so nothing is listening there.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(l.Addr().String())
	_ = l.Close()
	app := newTestApp()

	ep, ok := app.ProcessDomain("example.com:"+port, []net.IP{net.ParseIP("127.0.0.1")})["127.0.0.1"]
	if !ok || ep.alive {
		t.Errorf("endpoint = %+v (found %v), want a dead endpoint", ep, ok)
	}
}

func TestProcessDomainSkipsIPv6(t *testing.T) {
	app := newTestApp()
	if eps := app.ProcessDomain("example.com:1", []net.IP{net.ParseIP("::1")}); len(eps) != 0 {
		t.Errorf("ProcessDomain probed an IPv6 endpoint: %v", eps)
	}
}

func TestShowMetrics(t *testing.T) {
	app := newTestApp()
	app.services.Update([]byte(`{"https": {"domains": {"a.com": [], "b.com": [], "c.com": []}}}`))
	expiry := time.Unix(1893456000, 0)
	app.metrics.Set("a.com", Endpoints{"1.2.3.4": {CN: "a.com", AltNamesCount: 2, sha256: "ab", expiry: expiry, valid: true, alive: true}})
	app.metrics.Set("b.com", Endpoints{"5.6.7.8": {alive: false}})
	app.metrics.Set("c.com", Endpoints{})

	rec := httptest.NewRecorder()
	app.ShowMetrics(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	for _, want := range []string{
		`ssl_watch_domain_expiry{domain="a.com",service="https",sha="ab",ip="1.2.3.4",cn="a.com",alt_names="2",valid="true"} 1893456000`,
		`ssl_watch_domain_dead{domain="b.com",service="https",ip="5.6.7.8"} 1`,
		`ssl_watch_domain_unresolved{domain="c.com",service="https"} 1`,
	} {
		if !strings.Contains(body, want+"\n") {
			t.Errorf("metrics output is missing\n  %s\ngot:\n%s", want, body)
		}
	}
}

func TestS3ConfigsChanged(t *testing.T) {
	app := newTestApp()
	app.S3Configs = map[string]string{"ssl-watch/a.conf": `"etag-a"`, "ssl-watch/b.conf": `"etag-b"`}

	tests := []struct {
		name    string
		current map[string]string
		want    bool
	}{
		{"unchanged", map[string]string{"ssl-watch/a.conf": `"etag-a"`, "ssl-watch/b.conf": `"etag-b"`}, false},
		{"changed", map[string]string{"ssl-watch/a.conf": `"etag-a2"`, "ssl-watch/b.conf": `"etag-b"`}, true},
		{"added", map[string]string{"ssl-watch/a.conf": `"etag-a"`, "ssl-watch/b.conf": `"etag-b"`, "ssl-watch/c.conf": `"etag-c"`}, true},
		{"removed", map[string]string{"ssl-watch/a.conf": `"etag-a"`}, true},
	}
	for _, tt := range tests {
		if got := app.S3ConfigsChanged(tt.current); got != tt.want {
			t.Errorf("%s: S3ConfigsChanged() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestReloadConfigFromFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a.conf", `{"a": {"domains": {"a.com": []}}}`)
	write("b.conf", `{"b": {"domains": {"b.com": []}}}`)
	// Files without the suffix are ignored.
	write("c.json", `{"c": {"domains": {"c.com": []}}}`)

	app := newTestApp()
	app.config.ConfigDir = dir
	app.ReloadConfig()

	if got := len(app.services.ListDomains()); got != 2 {
		t.Errorf("ReloadConfig read %d domains, want 2: %v", got, app.services.ListDomains())
	}
	if _, ok := app.services.GetServiceName("c.com"); ok {
		t.Error("ReloadConfig read c.json, which lacks the .conf suffix")
	}
}

func TestScrape(t *testing.T) {
	port, _ := newTLSServer(t, 0)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, deadPort, _ := net.SplitHostPort(l.Addr().String())
	_ = l.Close()

	app := newTestApp()
	app.services.Update([]byte(`{"https": {
		"ips": {"local": ["127.0.0.1"]},
		"domains": {"example.com:` + port + `": ["local"], "dead.example.com:` + deadPort + `": ["127.0.0.1"]}
	}}`))
	app.scrape()

	if got := len(app.metrics.ListDomains()); got != 2 {
		t.Fatalf("after one scrape, metrics hold %d domains, want 2", got)
	}
	if eps, _ := app.metrics.Get("example.com:" + port); !eps["127.0.0.1"].alive || !eps["127.0.0.1"].valid {
		t.Errorf("example.com endpoints = %+v, want 127.0.0.1 alive and valid", eps)
	}
	if eps, _ := app.metrics.Get("dead.example.com:" + deadPort); eps["127.0.0.1"].alive {
		t.Errorf("dead.example.com endpoints = %+v, want 127.0.0.1 dead", eps)
	}
}

func TestResolveDomain(t *testing.T) {
	app := newTestApp()
	app.config.LookupTimeout = 500 * time.Millisecond

	found := false
	for _, ip := range app.ResolveDomain("localhost") {
		found = found || ip.IsLoopback()
	}
	if !found {
		t.Error("ResolveDomain(localhost) returned no loopback address")
	}

	// The .invalid top-level domain never resolves, so the lookup fails or times out.
	if ips := app.ResolveDomain("ssl-watch.invalid"); len(ips) != 0 {
		t.Errorf("ResolveDomain(ssl-watch.invalid) = %v, want none", ips)
	}
}
