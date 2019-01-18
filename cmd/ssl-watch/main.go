package main

import (
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	app := NewApp()
	sigHUP := make(chan os.Signal, 1)

	go app.updateMetrics()
	go func() {

		for {
			select {
			case <-sigHUP:
				app.log.Info("SIGHUP received, reloading configs")
				app.services.Flush()
				app.ReloadConfig()
				app.metrics.Flush()
			}
		}

	}()
	signal.Notify(sigHUP, syscall.SIGHUP)

	if app.config.S3Bucket != "" && app.config.AutoReload {
		app.log.Info("config check interval is " + app.config.ConfigCheckInterval.String())
		ticker := time.NewTicker(app.config.ConfigCheckInterval)
		go func() {
			for _ = range ticker.C {
				app.log.Debug("checking s3 configs for changes")
				current, err := app.GetS3ConfigHashes()
				if err == nil {
					if app.S3ConfigsChanged(current) {
						app.log.Info("s3 configs changed, reloading")
						app.services.Flush()
						app.ReloadConfig()
						app.metrics.Flush()
					}
				}
			}
		}()
	}
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
