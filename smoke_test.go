//go:build smoke

package main

import (
	"testing"
	"time"
)

// smokeDomains are public domains that ssl-watch monitors in production,
// keyed by the project that serves them.
var smokeDomains = map[string]string{
	"www.master-clock.us": "GPR",
	"www.gpreport.us":     "GPR",
	"favoriteshoes.us":    "GPR",
	"rptn.anchorfree.net": "ULA",
	"www.gvozdioptom.com": "SD",
}

// TestSmoke resolves each domain and probes every address with the TLS client
// this binary is built with. It catches endpoints that a Go upgrade can no longer
// reach, such as servers that only offer TLS 1.0 or RSA key exchange.
func TestSmoke(t *testing.T) {
	for domain, project := range smokeDomains {
		t.Run(project+"/"+domain, func(t *testing.T) {
			t.Parallel()
			app := newTestApp()
			app.config.ConnectionTimeout = 10 * time.Second
			app.config.LookupTimeout = 5 * time.Second

			eps := app.ProcessDomain(domain, nil)
			if len(eps) == 0 {
				t.Fatal("the domain has no IPv4 addresses")
			}
			for ip, ep := range eps {
				switch {
				case !ep.alive:
					t.Errorf("%s: TLS handshake failed", ip)
				case !ep.valid:
					t.Errorf("%s: the certificate does not cover the domain (CN %q)", ip, ep.CN)
				case time.Until(ep.expiry) < 0:
					t.Errorf("%s: the certificate expired on %s", ip, ep.expiry.Format(time.DateOnly))
				default:
					t.Logf("%s: OK, CN %q, expires %s", ip, ep.CN, ep.expiry.Format(time.DateOnly))
				}
			}
		})
	}
}
