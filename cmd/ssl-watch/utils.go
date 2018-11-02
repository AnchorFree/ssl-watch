package main

import (
	"encoding/json"
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
	log.Init("sslwatch", app.config.DebugMode, false, nil)
	app.log = log
	reloadConfig(app)
	return app
}

func reloadConfig(app *App) {

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
			json.Unmarshal([]byte(raw), &app.Domains)
		}
	}

	if len(app.Domains) == 0 {
		app.log.Fatal("no configs provided", errors.New("no config files"))
	}

}

func resolveDomain(app *App, domain string, timeout time.Duration) []net.IP {

	timer := time.NewTimer(timeout)
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

func IsIPv4(address string) bool {
	return strings.Count(address, ":") < 2
}

func StrToIp(IPList []string) []net.IP {

	ips := []net.IP{}

	for _, ipString := range IPList {
		ip := net.ParseIP(ipString)
		if ip != nil {
			ips = append(ips, ip)
		}
	}
	return ips

}
