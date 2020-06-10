ssl-watch — a tool to monitor SSL certificates expiration
=========================================================

Table of Contents
-----------------
* [Description](#description)
* [Configuration](#configuration)
* [Operation](#operation)
* [Exported metrics](#exported-metrics)
* [Credits](#credits)

Description
-------------

`ssl-watch` is a golang daemon to monitor expiration dates
of SSL certificates and export this data as [prometheus](https://prometheus.io/) metrics.

You provide one or more configuration files listing domain names to monitor
and optionally a list of IP addresses for each domain. Every SCRAPE_INTERVAL 
`ssl-watch` examines certificates for each domain at each IP endpoint and exports 
prometheus metrics with expiration date and some additional information. 

Note that `ssl-watch` does **not** try to validate the whole certificate chain, the only
thing it does in terms of validation is checking at each IP endpoint whether 
Common Name of the certificate or one of its' SANs has the domain name defined in the config.
If it does, then SSLWATCH sets `valid="true"` label in prometheus metrics for the domain,
otherwise it will be set to `valid="false"`.
 
Configuration
-------------

`ssl-watch` is configured with environment variables:

* **SSLWATCH_CONFIG_DIR**  
Path to the directory with domains config files. Default is **/etc/ssl-watch**.
Each file in the directory should have a `.conf` suffix (configurable via **SSLWATCH_CONFIG_FILE_SUFFIX**), and be in JSON format, 
listing domain names to be inspected and their optional IP endpoints.
Domain names and their IP endpoints should be grouped into "services" blocks:

```json
{ 
  "mailCerts" :
    { 
      "ips" : { "set1" : [ "127.0.0.1", "127.0.0.2", "127.0.0.3" ], "set2": [ "127.0.0.4" ] },
      "domains" : { "example.com:465": [], "sample.net:993": [ "set1", "set2", "127.0.0.5" ] } 
    },
  
  "https" : 
    {
      "domains" : { "jack.com": [], "daniels.org:8443": [], "absinth.io": [ "192.168.0.7", "192.168.0.8" ] } 
    }
}
```

The exported metrics will have `service` label set to `mailCerts` for `example.com` and `sample.net` domains,
and to `https` for `jack.com`,`daniels.org` and `absinth.io` domains.

Files in the directory that don't have `.conf` suffix are ignored.
When there are no IP addresses provided for a domain, `ssl-watch` will try to resolve
it, and connect to all IP addresses the domain name resolves to. As seen from the example
above, you can also provide named IP sets and use them as endpoints for all or some of domains.
Note that a particular named IP set is only valid within a service block where it was declared, i.e.
in the example above you can't use `set1` or `set2` as domain endpoints in `https` service.

You can also set **SSLWATCH_CONFIG_DIR** to an AWS S3 bucket path, for ex.: `s3://my-s3-bucket/some/dir`.
In this case `ssl-watch` will read configs from S3 bucket.

* **SSLWATCH_CONFIG_FILE_SUFFIX**  
Default is **.conf**

* **SSLWATCH_AUTO_RELOAD**  
When you set **SSLWATCH_CONFIG_DIR** to an s3 path, this setting controls
whether `ssl-watch` should reload configs from s3 automatically if any of them have been changed.
If set to `true`, `ssl-watch` will check for config changes every **SSLWATCH_CONFIG_CHECK_INTERVAL**, and reload them upon any changes.
Default is **true**

* **SSLWATCH_CONFIG_CHECK_INTERVAL**  
Default is **5m**

* **SSLWATCH_SCRAPE_INTERVAL**  
Interval between checking remote ssl endpoints. Default is **60s**

* **SSLWATCH_CONNECTION_TIMEOUT**  
TCP connection timeout. Default is **10s**

* **SSLWATCH_LOOKUP_TIMEOUT**  
Timeout for resolving a domain name. Default is **5s**

* **SSLWATCH_PORT**  
Port on which to start http server to serve metrics. Default is **9105**.
Metrics will be available at `http://*:9105/metrics`.

* **SSLWATCH_DEBUG_MODE**  
Turns on debug level logging. Default is **false**.

Operation
---------

Upon receiving a SIGHUP signal `ssl-watch` flushes current metrics
and reloads config files.

Exported metrics
----------------

| Name | Type | Labels | Remarks |
| ---- | ---- | ------ | ------- |
| ssl_watch_domain_expiry | gauge | domain, service, ip, cn, alt_names, valid | expiration date in Unix time. `service` is service name from the config, `cn` is common name of the certificate, `sha` is a SHA256 fingerprint of the certificate, `alt_names` shows count of SANs in the certificate, `valid` will be set to true if certificates's CommonName or one of its' SANs has `domain` defined.|
| ssl_watch_domain_dead | gauge | domain, service, ip | this metric will be set to 1 when SSLWATCH fails to connect to an IP endpoint |
| ssl_watch_domain_unresolved | gauge | domain, service | this metric will be set to 1 when SSLWATCH fails to resolve a domain |


Credits
-------

`ssl-watch` is inspired and loosely based on the code of [check-ssl](https://github.com/wycore/check-ssl) project.
