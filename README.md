ssl-watch — a tool to monitor ssl certificates expiration
=======================================

Configuration
-------------

SSLWATCH is configured with environment variables:

* **SSLWATCH_CONFIG_DIR**  
Path to the directory with domains config files. Default is **/etc/ssl-watch**.
Each file in the directory should have a `.conf` suffix, and be in JSON format, 
listing domain names to be inspected and their optional IP endpoints:
```
{ "example.com": [], "my.secret.domain.com" : ["127.0.0.1", "127.0.0.33", "127.0.0.4" ], "google.com" : [] }

```
You can also declare a list of IPs and use it for other domains:
```
{ "myIPSet": ["192.168.0.1", "192.168.0.2"], "example.com": ["myIPset"], "sub.example.com": ["myIPSet"], "google.com": [] }
```

When you declare a list its' name should not contain dots -- that's how
SSLWATCH determines if this is a domain name or a list declaration. Similarly, when the first 
IP address provided for a domain does not have a dot, SSLWATCH considers it to be a reference to
an IP list, and "resolves" it.

Files in the directory that don't have `.conf` suffix are ignored.
When there are no IP addresses provided for a domain, SSLWATCH will try to resolve
it, and connect to all IP addresses the domain name resolves to.

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

Operation
---------

Upon receiving a SIGHUP signal SSLWATCH flushes current metrics
and reloads config files.

Exported metrics
----------------

| Name | Type | Labels | Remarks |
| ---- | ---- | ------ | ------- |
| ssl_watch_domain_expiry | gauge | domain, ip, cn, alt_names, valid | expiration date in Unix time |
| ssl_watch_domain_dead | gauge | domain, ip | this metric will be set to 1 when SSLWATCH fails to connect to an ip |
| ssl_watch_domain_unresolved | gauge | domain | this metric will be set to 1 when SSLWATCH can't resolve a domain |

