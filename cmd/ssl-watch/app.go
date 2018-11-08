package main

import (
	"crypto/tls"
	"errors"
	"github.com/anchorfree/golang/pkg/jsonlog"
	"github.com/kelseyhightower/envconfig"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"time"
)

func NewApp() *App {

	log := &jsonlog.StdLogger{}
	log.Init("sslwatch", false, false, nil)

	app := &App{config: Config{}}
	err := envconfig.Process("sslwatch", &app.config)
	if err != nil {
		log.Fatal("failed to initialize", err)
	}

	app.metrics = Metrics{mutex: sync.RWMutex{}, db: map[string]Endpoints{}}
	app.domains = Domains{mutex: sync.RWMutex{}, db: map[string][]string{}}
	log.Init("sslwatch", app.config.DebugMode, false, nil)
	app.log = log
	app.ReloadConfig()
	return app
}

func (app *App) ReloadConfig() {

	files, err := ioutil.ReadDir(app.config.ConfigDir)
	if err != nil {
		app.log.Fatal("can't read config files dir", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".conf") {
			raw, err := ioutil.ReadFile(app.config.ConfigDir + "/" + file.Name())
			if err != nil {
				app.log.Error("can't read config file "+file.Name(), err)
			}
			app.domains.Update(raw)
		}
	}

	if len(app.domains.db) == 0 {
		app.log.Fatal("no configs provided", errors.New("no config files"))
	}
	app.log.Debug("domains read from configs", app.domains.List())

}

func (app *App) ProcessDomain(domain string, ips []net.IP) Endpoints {

	host, port, err := net.SplitHostPort(domain)
	if err != nil {
		host = domain
		port = "443"
	}

	if len(ips) == 0 {
		ips = app.ResolveDomain(host)
	}
	endpoints := Endpoints{}

	for _, ip := range ips {

		dialer := net.Dialer{Timeout: app.config.ConnectionTimeout, Deadline: time.Now().Add(app.config.ConnectionTimeout + 5*time.Second)}

		if IsIPv4(ip.String()) {

			endpoint := Endpoint{}
			connection, err := tls.DialWithDialer(&dialer, "tcp", ip.String()+":"+port, &tls.Config{ServerName: host, InsecureSkipVerify: true})
			if err != nil {
				app.log.Error(ip.String(), err)
				endpoint.alive = false
				endpoints[ip.String()] = endpoint
				continue
			}

			cert := connection.ConnectionState().PeerCertificates[0]
			endpoint.alive = true
			endpoint.expiry = cert.NotAfter
			endpoint.CN = cert.Subject.CommonName
			endpoint.AltNamesCount = len(cert.DNSNames)
			err = cert.VerifyHostname(host)
			if err != nil {
				endpoint.valid = false
			} else {
				endpoint.valid = true
			}
			connection.Close()
			endpoints[ip.String()] = endpoint
		}
	}
	return endpoints

}

func (app *App) ResolveDomain(domain string) []net.IP {

	timer := time.NewTimer(app.config.LookupTimeout)
	ch := make(chan []net.IP, 1)

	go func() {
		r, err := net.LookupIP(domain)
		if err != nil {
			app.log.Error("failed to lookup "+domain, err)
			return
		}
		ch <- r
	}()

	select {
	case ips := <-ch:
		return ips
	case <-timer.C:
	}
	return make([]net.IP, 0)

}

func (app *App) updateMetrics() {

	ticker := time.NewTicker(app.config.ScrapeInterval)
	defer ticker.Stop()

	for ; true; <-ticker.C {
		domains := app.domains.List()
		app.log.Debug("current domains", domains)
		for _, domain := range domains {
			app.log.Debug("processing domain " + domain)
			addrSet := app.domains.GetIPs(domain)
			eps := app.ProcessDomain(domain, StrToIp(addrSet))
			app.metrics.Set(domain, eps)
		}
	}

}
