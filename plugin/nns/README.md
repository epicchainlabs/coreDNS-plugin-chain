# nns   

## Name

*nns* - enables serving data from [neo blockchain](https://neo.org/).

## Description

The nns plugin tries to get value from TXT records in provided neo node 
(lookup in [NNS smart contract](https://docs.neo.org/docs/en-us/reference/nns.html)).
You can specify NNS domain to map DNS domain from request (default no mapping).

## Syntax

``` txt
nns NEO_N3_CHAIN_ENDPOINT CONTRACT_ADDRESS [NNS_DOMAIN]
```

`CONTRACT_ADDRESS` - hex-encoded contract script hash. The value is required, but if you are using the NNS contract 
in side-chain, you can place `-` and the plugin will use contract with ID 1 as NNS.
``` txt
nns NEO_N3_CHAIN_ENDPOINT - [NNS_DOMAIN]
```

You can specify more than one contract. They will be handled as follows:

* Constructing the resulting record set by taking the content from each contract and overriding (conflicting records) 
in the order of appearance in config file.
* Using `AXFR` request the `SOA` record taking from the original (first in the order of appearance) zone.

## Examples

In this configuration, first we try to find the result in the provided neo node and forward 
requests to 8.8.8.8 if the neo request fails.

``` corefile
. {
  nns http://localhost:30333 acf433b55b75907fd80e8c90c9c42140992c8240
  forward . 8.8.8.8
}
```

This example shows how to map `containers.testnet.fs.neo.org` dns domain to `containers` nns domain 
(so request for `nicename.containers.testnet.fs.neo.org` will transform to `nicename.containers`).
It also enables zone transfer support:

``` corefile
containers.testnet.fs.neo.org {
  nns http://morph-chain.neofs.devenv:30333 - containers
  transfer {
      to *
  }
}
```

If there is no domain filter in config:

``` corefile
. {
  nns http://morph-chain.neofs.devenv:30333 - containers
}
```

Request for `nicename.containers.testnet.fs.neo.org` will transform to `nicename.containers.testnet.fs.neo.org.containers`.
