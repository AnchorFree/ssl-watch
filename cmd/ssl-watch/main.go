package main

import (
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func updateMetrics(app *App, ticker *time.Ticker, firstRun, quit chan bool) {

	update := func() {

		domains := app.domains.List()
		app.log.Debug("current domains", domains)
		for _, domain := range domains {
			app.log.Debug("processing domain " + domain)
			addrSet := app.domains.GetIPs(domain)
			eps := app.ProcessDomain(domain, StrToIp(addrSet))
			app.metrics.Set(domain, eps)
		}

	}

	for {
		select {
		case <-firstRun:
			update()
		case <-ticker.C:
			update()
		case <-quit:
			ticker.Stop()
			quit <- true
			return
		}
	}

}

func main() {

	app := NewApp()

	firstRun := make(chan bool, 1)
	restart := make(chan bool, 1)
	quit := make(chan bool, 1)
	sigHUP := make(chan os.Signal, 1)

	go func() {

		for {
			select {
			case <-restart:
				ticker := time.NewTicker(app.config.ScrapeInterval)
				go updateMetrics(app, ticker, firstRun, quit)
				firstRun <- true

			case <-sigHUP:
				app.log.Info("SIGHUP received, reloading configs")
				quit <- true
				<-quit
				app.domains.Flush()
				app.metrics.Flush()
				app.ReloadConfig()
				restart <- true
			}
		}

	}()
	signal.Notify(sigHUP, syscall.SIGHUP)

	app.log.Info("config dir is set to be at " + app.config.ConfigDir)
	app.log.Info("scrape interval is " + app.config.ScrapeInterval.String())
	app.log.Info("connection timeout is " + app.config.ConnectionTimeout.String())
	app.log.Info("lookup timeout is " + app.config.LookupTimeout.String())
	app.log.Info("starting http server on port " + app.config.Port)
	restart <- true

	rtr := mux.NewRouter()
	rtr.HandleFunc("/metrics", app.ShowMetrics).Methods("GET")
	http.Handle("/", rtr)
	app.log.Fatal("http server stopped", http.ListenAndServe(":"+app.config.Port, nil))

}
