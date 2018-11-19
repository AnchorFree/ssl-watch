ssl-watch — a tool to monitor SSL certificates expiration
=========================================================

Description
-------------

SSLWATCH is a golang daemon to monitor expiry dates
of SSL certificates and export this data as prometheus metrics.

You provide one or more configuration files listing domain names to monitor
and optionally a list of IP addresses for each domain. Every SCRAPE_INTERVAL 
SSLWATCH examines certificates for each domain at each IP endpoint and exports 
prometheus metrics with expiration date and some additional information. 

Note that SSLWATCH does **not** try to validate the whole certificate chain, the only
thing it does in terms of validation is checking at each IP endpoint whether 
Common Name of the certificate or one of its' SANs has the domain name defined in the config.
If it does, than SSLWATCH sets `valid="true"` label in prometheus metrics for this domain,
otherwise it will be set to `valid="false"`.
 
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

If you need you can also specify a non-standard port (by default **443** is used):
```
{ "imap.gmail.com:993": [], "mail.example.com:465": [] }
```

You can also declare a list of IPs and use it for other domains:
```
{ "myIPSet": ["192.168.0.1", "192.168.0.2"], "example.com": ["myIPset"], "sub.example.com": ["myIPSet"], "google.com": [] }
```

When you declare a list its' name should not contain dots -- that's how
SSLWATCH determines if this is a domain name or a list declaration. Similarly, when the first 
IP address provided for a domain does not have a dot, SSLWATCH considers it to be a reference to
an IP list, and "resolves" it. At the moment you can't use more than one IP list per domain,
i.e. the following `{ "list1": ["192.168.0.1"], "list2": ["192.168.0.2"], "example.com": ["list1","list2"] }`
config is not permitted.

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

