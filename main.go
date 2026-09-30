package main

import (
	"github.com/gorilla/mux"
	"go.uber.org/zap"
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
			for range ticker.C {
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
	// Bound every phase of a request, so a slow or idle client cannot hold a connection open.
	srv := &http.Server{
		Addr:              ":" + app.config.Port,
		Handler:           rtr,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	app.log.Fatal("http server stopped", zap.Error(srv.ListenAndServe()))

}
