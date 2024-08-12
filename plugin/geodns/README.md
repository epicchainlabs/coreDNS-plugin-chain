# geodns

## Name

*geodns* - Lookup maxmind geoip2 databases using the response servers IP and filter by distance to the client IP.

## Description

The geodns plugin filter response dns records (types: `A, AAAA`) and transfer only closest to the client. 
Plugin supports `city` and `country` type db. If directory contains more than one db each type, the last one is used.
You can specify max allowed records to response (default is 1).

## Syntax

``` txt
geodns GEOIP_DATABASES_DIR_PATH [MAX_RECORDS]
```

## Examples

In this configuration, we will filter `A` and `AAAA` records that nns plugin found in the NEO blockchain.

``` corefile
. {
   geodns testdata/
   nns http://localhost:30333
}
```
