ssl-watch — a tool to monitor ssl certificates expiration
=======================================

Configuration
-------------

SSLWATCH is configured with environment variables:

* **SSLWATCH_CONFIG_FILE**  
Path to the domains config file. The file should be in JSON format,
listing domain names to be inspected and their optional IP endpoints:
```
{ "example.com": [], "my.secret.domain.com" : ["127.0.0.1", "127.0.0.33", "127.0.0.4" ], "google.com" : [] }

```
When there is no IP addresses provided for a domain, SSLWATCH will try to resolve
it, and connect to all IP addresses the domain name resolves to.
Default is **/etc/ssl-watch.conf**.

* **SSLWATCH_SCRAPE_INTERVAL**  
Interval between checking remote ssl endpoints. Default is **60s**

* **SSLWATCH_CONNECTION_TIMEOUT**  
Timeout for the container inspect API call. Default is **10s**

* **SSLWATCH_LOOKUP_TIMEOUT**  
Timeout for the container inspect API call. Default is **5s**

* **SSLWATCH_PORT**  
Port on which to start http server to serve metrics. Default is **9105**.
Metrics will be available at `http://localhost:9105/metrics`.

* **SSLWATCH_DEBUG_MODE**  
Turns on debug level logging. Default is **false**.

Exported metrics
----------------

| Name | Type | Labels | Remarks |
| ---- | ---- | ------ | ------- |
| ssl_watch_domain_expiry | gauge | domain, ip, cn, alt_names, valid | expiration date in Unix time |
| ssl_watch_domain_dead | gauge | domain, ip | this metric will be set to 1 when SSLWATCH fails to connect to an ip |
| ssl_watch_domain_unresolved | gauge | domain | this metric will be set to 1 when SSLWATCH can't resolve a domain |

