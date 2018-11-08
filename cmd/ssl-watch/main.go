package main

import (
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *App) updateMetrics() {

	for {
		domains := app.domains.List()
		app.log.Debug("current domains", domains)
		for _, domain := range domains {
			app.log.Debug("processing domain " + domain)
			addrSet := app.domains.GetIPs(domain)
			eps := app.ProcessDomain(domain, StrToIp(addrSet))
			app.metrics.Set(domain, eps)
		}
		time.Sleep(app.config.ScrapeInterval)
	}

}

func main() {

	app := NewApp()
	sigHUP := make(chan os.Signal, 1)

	go app.updateMetrics()
	go func() {

		for {
			select {
			case <-sigHUP:
				app.log.Info("SIGHUP received, reloading configs")
				app.domains.Flush()
				app.metrics.Flush()
				app.ReloadConfig()
			}
		}

	}()
	signal.Notify(sigHUP, syscall.SIGHUP)

	app.log.Info("config dir is set to be at " + app.config.ConfigDir)
	app.log.Info("scrape interval is " + app.config.ScrapeInterval.String())
	app.log.Info("connection timeout is " + app.config.ConnectionTimeout.String())
	app.log.Info("lookup timeout is " + app.config.LookupTimeout.String())
	app.log.Info("starting http server on port " + app.config.Port)

	rtr := mux.NewRouter()
	rtr.HandleFunc("/metrics", app.ShowMetrics).Methods("GET")
	http.Handle("/", rtr)
	app.log.Fatal("http server stopped", http.ListenAndServe(":"+app.config.Port, nil))

}
