[![EpicChain](https://epic-chain.org/images/EpicChain_Logo.png)](https://epic-chain.org)

[![Documentation](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/epicchainlabs/coreDNS-plugin-chain)
[![Build Status](https://img.shields.io/travis/epicchain/epicchain/master.svg?label=build)](https://travis-ci.org/epicchain/epicchain)
[![Fuzzit](https://app.fuzzit.dev/badge?org_id=epicchain&branch=master)](https://fuzzit.dev)
[![Code Coverage](https://img.shields.io/codecov/c/github/epicchain/epicchain/master.svg)](https://codecov.io/github/epicchain/epicchain?branch=master)
[![Docker Pulls](https://img.shields.io/docker/pulls/epicchain/epicchain.svg)](https://hub.docker.com/r/epicchain/epicchain)
[![Go Report Card](https://goreportcard.com/badge/github.com/epicchainlabs/coreDNS-plugin-chain)](https://goreportcard.com/report/epicchain/epicchain)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1250/badge)](https://bestpractices.coreinfrastructure.org/projects/1250)

## EpicChain Overview

EpicChain is an advanced blockchain platform designed to provide a flexible and high-performance environment for decentralized applications and services. Developed using the Go programming language, EpicChain utilizes a modular plugin architecture that allows for extensive customization and scalability.

### Key Features

- **Modular Architecture:** EpicChain's plugin-based system allows users to easily extend functionality. You can integrate new features or customize existing ones by writing and adding plugins.
- **Protocol Support:** EpicChain supports various communication protocols including traditional DNS over UDP/TCP, DNS over TLS (DoT), DNS over HTTP/2 (DoH), and gRPC.
- **Dynamic Functionality:** The platform supports numerous functions such as serving DNS zone data, DNSSEC, load balancing, caching, and more. With EpicChain, you can handle complex DNS queries, manage zone transfers, and ensure robust security and performance.
- **Advanced Configuration:** EpicChain allows for detailed configuration including load balancing, query logging, error handling, and integration with various backends such as etcd and Kubernetes.
- **High Performance:** The platform is designed for efficiency with features like caching, load balancing, and support for high-throughput transactions.

### Installation and Setup

#### Building from Source

To build EpicChain from source, ensure you have a working Go environment (version 1.12 or higher). Follow these steps:

```bash
$ git clone https://github.com/epicchainlabs/coreDNS-plugin-chain
$ cd coreDNS-plugin-chain
$ make
```

This will compile the `epicchain` binary.

#### Building with Docker

If you prefer using Docker, you can build EpicChain without setting up a Go environment:

```bash
$ docker run --rm -i -t -v $PWD:/v -w /v golang:1.16 make
```

This command will create the `epicchain` binary within the Docker container.

### Configuration Examples

EpicChain's configuration is managed through a file named `Epicfile`. Here are some examples to get you started:

- **Basic Setup:**

  To run EpicChain with basic logging and serve requests on port 53:

  ```text
  :53 {
      whoami
      log
  }
  ```

- **Custom Port:**

  If port 53 is occupied, you can configure EpicChain to use port 1053:

  ```text
  :1053 {
      whoami
      log
  }
  ```

- **Forwarding Queries:**

  To forward all queries to an upstream server (e.g., Google DNS at 8.8.8.8):

  ```text
  :53 {
      forward . 8.8.8.8:53
      log
  }
  ```

- **Serving DNSSEC-Signed Zones:**

  To serve DNSSEC-signed data for `example.org`:

  ```text
  example.org:1053 {
      file /var/lib/epicchain/example.org.signed
      transfer {
          to * 2001:500:8f::53
      }
      errors
      log
  }
  ```

- **Advanced Configurations:**

  To handle different types of queries and protocols:

  ```text
  tls://example.org grpc://example.org {
      whoami
  }

  https://example.org {
      whoami
      tls mycert mykey
  }
  ```

### Community and Support

Stay connected with the EpicChain community and get support:

- **GitHub:** [EpicChain GitHub Repository](https://github.com/epicchainlabs/coreDNS-plugin-chain)
- **Slack:** Join us on Slack at #epicchain [here](https://slack.epic-chain.org)
- **Website:** [EpicChain Official Website](https://epic-chain.org)
- **Blog:** [EpicChain Blog](https://blog.epic-chain.org)
- **Twitter:** [@EpicChain](https://twitter.com/EpicChainLabs)
- **Mailing List:** [epicchain-discuss@googlegroups.com](mailto:epicchain-discuss@googlegroups.com)

### Contribution Guidelines

Interested in contributing? Check out our [contribution guidelines](CONTRIBUTING.md) to learn how you can get involved.

### Deployment

For detailed deployment examples, including systemd configurations, visit our [deployment repository](https://github.com/epicchainlabs/deployment).

### Deprecation Policy

EpicChain follows a structured deprecation policy to manage backwards incompatible changes:

1. **Announcement:** Notify users about upcoming incompatible changes in the release notes.
2. **Implementation:** Introduce changes in a minor release, ensuring backward compatibility.
3. **Removal:** Remove support for deprecated features in a subsequent patch release.

### Security

**Security Audit:** EpicChain undergoes regular security audits to ensure robustness. Review our [audit report](https://epic-chain.org/assets/SECURITY-REPORT.pdf) for details.

**Reporting Vulnerabilities:** If you discover a security issue, please report it privately to `security@epic-chain.org`. Your contributions to our security are greatly appreciated.

For detailed security practices, refer to our [security documentation](https://github.com/epicchainlabs/coreDNS-plugin-chain/blob/master/SECURITY.md).

