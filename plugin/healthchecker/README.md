# healthchecker

## Name

*healthchecker* - filters records with unhealthy IPs (types: `A, AAAA`).

## Description

A healthchecker plugin filters input DNS records and returns healthy records. To response fast, it stores records and 
their statuses in LRU cache and responses in the following way:
1. if the record is not found in the cache the plugin returns the records as healthy, triggers check and puts it into 
the cache
2. if the record is found in the cache the plugin returns the record if it's healthy

Also, the plugin can be configured, what record names will be checked. If name filters are set, the plugin will check  
and store in cache only records which suite with the filters, otherwise the record will always be returned 
as healthy. If the filter is not set, the plugin will check and store all records.

## Syntax

### Common
- `CACHE_SIZE` -- maximum number of records in cache
- `HEALTHCHECK_INTERVAL` -- time interval of updating status of records in cache in duration format
- `REGEXP_FILTER` -- any valid regexp pattern to filter which records will be cached (also can be `@` which means origin).
- `[ADDITIONAL_REGEXP_FILTERS... ]` -- optional filters (the same name as `REGEXP_FILTER`).
  A record will be cached if it matches any filter.

``` txt
healthchecker HEALTHCHECK_METHOD CACHE_SIZE HEALTHCHECK_INTERVAL REGEXP_FILTER [ADDITIONAL_REGEXP_FILTERS... ]
```

- `HEALTHCHECK_METHOD` -- method of checking of nodes: `http` and `icmp` is implemented.  

### HTTP

HTTP method can be configured in the following block format (all block params can be safely omitted): 
```
http CACHE_SIZE HEALTHCHECK_INTERVAL REGEXP_FILTER {
  port PORT 
  timeout TIMEOUT_IN_MS
}
```

- `PORT` -- port of remote endpoint to make http request (default: 80)
- `TIMEOUT_IN_MS` -- request timeout to remote endpoint (default: 2s)


### ICMP

ICMP method can be configured in the following block format (all block params can be safely omitted): 
```
icmp CACHE_SIZE HEALTHCHECK_INTERVAL REGEXP_FILTER {
  privileged 
  timeout TIMEOUT_IN_MS
}
```

- `privileged` -- if provided, then `ip4:icmp` or `ip6:ipv6-icmp` network is used (otherwise `udp4` or `udp6` network is used). 
Make sure you run Coredns as root.
- `TIMEOUT_IN_MS` -- timeout of waiting remote endpoint echo reply (default: 2s)



## Examples

In this configuration, we will filter `A` and `AAAA` records, store maximum 1000 records in cache, and start recheck of 
each record in cache for every 3 seconds via http client. The plugin will check records with name 
fs.neo.org (`@` in config) or cdn.fs.neo.org (`^cdn\.fs\.neo\.org` in config).
HTTP requests to check and update statuses of IPs will use default 80 port and wait for default 2 seconds.
``` corefile
fs.neo.org. {
    healthchecker http 1000 1s @ ^cdn\.fs\.neo\.org
    file db.example.org fs.neo.org
}
```

The same as above but port and timeout for HTTP client are set.
``` corefile
fs.neo.org. {
    healthchecker http 1000 1s @ ^cdn\.fs\.neo\.org {
      port 80
      timeout 3s
    }
    file db.example.org fs.neo.org
}
```

Default ICMP checker:
```
fs.neo.org. {
    healthchecker icmp 1000 1s @ ^cdn\.fs\.neo\.org 
    file db.example.org fs.neo.org
}
```

Privileged ICMP checker and custom timeout:
```
fs.neo.org. {
    healthchecker icmp 1000 1s @ ^cdn\.fs\.neo\.org {
      privileged
      timeout 3s
    }
    file db.example.org fs.neo.org
}
```
