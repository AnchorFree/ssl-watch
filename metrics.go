package main

import (
	"net/http"
	"strconv"
	"strings"
)

// ShowMetrics outputs metrics for Prometheus
func (app *App) ShowMetrics(w http.ResponseWriter, r *http.Request) {

	var buf1, buf2, buf3 strings.Builder

	domains := app.metrics.ListDomains()

	buf1.WriteString("# TYPE ssl_watch_domain_expiry gauge\n")
	buf2.WriteString("# TYPE ssl_watch_domain_dead gauge\n")
	buf3.WriteString("# TYPE ssl_watch_domain_unresolved gauge\n")

	for _, domain := range domains {

		eps, _ := app.metrics.Get(domain)
		service, _ := app.services.GetServiceName(domain)
		if len(eps) == 0 {
			buf3.WriteString("ssl_watch_domain_unresolved{domain=\"" + domain + "\",service=\"" + service + "\"} 1\n")
		}
		for ip, ep := range eps {
			if ep.alive {
				buf1.WriteString("ssl_watch_domain_expiry{domain=\"" + domain + "\",service=\"" + service + "\",sha=\"" + string(ep.sha256) + "\",ip=\"" + ip + "\",cn=\"" + ep.CN + "\",alt_names=\"" + strconv.Itoa(ep.AltNamesCount) + "\",valid=\"" + strconv.FormatBool(ep.valid) + "\"} " + strconv.FormatInt(ep.expiry.Unix(), 10) + "\n")
			} else {
				buf2.WriteString("ssl_watch_domain_dead{domain=\"" + domain + "\",service=\"" + service + "\",ip=\"" + ip + "\"} 1\n")
			}
		}
	}

	_, _ = w.Write([]byte(buf1.String() + buf2.String() + buf3.String()))

}
