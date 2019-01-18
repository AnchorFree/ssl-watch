package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"github.com/anchorfree/golang/pkg/jsonlog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/kelseyhightower/envconfig"
	"io"
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
	app.services = Services{mutex: sync.RWMutex{}, db: map[string]Service{}, reverseMap: map[string]string{}}
	log.Init("sslwatch", app.config.DebugMode, false, nil)
	app.log = log
	app.config.S3Bucket, app.config.S3Key = ParseS3Path(app.config.ConfigDir)
	if app.config.S3Bucket != "" {
		err := app.CreateS3Session()
		if err != nil {
			app.log.Fatal("can't init S3 session", err)
		}
	}
	app.ReloadConfig()
	return app

}

func (app *App) ReloadConfig() {

	if app.config.S3Bucket != "" {
		app.reloadConfigFromS3()
	} else {
		app.reloadConfigFromFiles()
	}

}

func (app *App) reloadConfigFromFiles() {

	files, err := ioutil.ReadDir(app.config.ConfigDir)
	if err != nil {
		app.log.Error("can't read config files dir", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), app.config.ConfigFileSuffix) {
			raw, err := ioutil.ReadFile(app.config.ConfigDir + "/" + file.Name())
			if err == nil {
				app.services.Update(raw)
			} else {
				app.log.Error("can't read config file "+file.Name(), err)
			}
		}
	}

	if len(app.services.db) == 0 {
		app.log.Fatal("no configs provided", errors.New("no config files"))
	}
	app.log.Debug("domains read from configs", app.services.ListDomains())

}

func (app *App) reloadConfigFromS3() {

	configHashes, err := app.GetS3ConfigHashes()
	if err != nil {
		app.log.Error("can't stat objects in s3 bucket", err)
	}

	for config := range configHashes {
		raw, err := app.ReadS3File(config)
		if err == nil {
			app.services.Update(raw)
		} else {
			app.log.Error("failed to read s3 config "+config, err)
		}
	}

	if len(app.services.db) == 0 {
		app.log.Fatal("no configs provided", errors.New("no config files"))
	}
	app.S3Configs = configHashes
	app.log.Debug("domains read from configs", app.services.ListDomains())

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

			sha := sha1.New()
			cert := connection.ConnectionState().PeerCertificates[0]
			endpoint.alive = true
			sha.Write(cert.Raw)
			endpoint.sha1 = hex.EncodeToString(sha.Sum(nil))
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
		domains := app.services.ListDomains()
		app.log.Debug("current domains", domains)
		for _, domain := range domains {
			app.log.Debug("processing domain " + domain)
			ips := app.services.GetIPs(domain)
			eps := app.ProcessDomain(domain, StrToIp(ips))
			app.metrics.Set(domain, eps)
		}
	}

}

func (app *App) CreateS3Session() error {

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(app.config.S3Region)},
	)

	if err != nil {
		return err
	}

	app.S3Session = sess
	return nil

}

func (app *App) ReadS3File(key string) ([]byte, error) {

	results, err := s3.New(app.S3Session).GetObject(&s3.GetObjectInput{
		Bucket: aws.String(app.config.S3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer results.Body.Close()

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, results.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil

}

func (app *App) GetS3ConfigHashes() (map[string]string, error) {

	hashes := map[string]string{}

	svc := s3.New(app.S3Session)

	resp, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: aws.String(app.config.S3Bucket), Prefix: aws.String(app.config.S3Key)})
	if err != nil {
		return hashes, err
	}
	for _, item := range resp.Contents {
		if strings.HasSuffix(*item.Key, app.config.ConfigFileSuffix) {
			hashes[*item.Key] = *item.ETag
		}
	}
	return hashes, nil

}

func (app *App) S3ConfigsChanged(current map[string]string) bool {

	changed := false
	for k, v := range current {
		v1, ok := app.S3Configs[k]
		if !ok {
			app.log.Debug(k + " config has been added")
			changed = true
		} else {
			if v1 != v {
				app.log.Debug(k + " config has been changed")
				changed = true
			}
		}

	}

	for k := range app.S3Configs {
		_, ok := current[k]
		if !ok {
			app.log.Debug(k + " config has been removed")
			changed = true
		}

	}
	return changed

}
