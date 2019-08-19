![NATS](logos/large-logo.png)

# NATS Replicator

[![License][License-Image]][License-Url]
[![ReportCard][ReportCard-Image]][ReportCard-Url]
[![Build][Build-Status-Image]][Build-Status-Url]

This project implements a multi-connector bridge, copier, replicator between NATS and NATS streaming endpoints.

## Features

* Arbitrary subjects in NATS, wildcards for incoming messages
* Arbitrary channels in NATS streaming
* Optional durable subscriber names for streaming
* Configurable std-out logging
* A single configuration file, with support for reload
* Optional SSL to/from NATS and NATS streaming
* HTTP/HTTPS-based monitoring endpoints for health or statistics

## Overview

The replicator runs as a single process with a configured set of connectors mapping a between a NATS subject or a NATS streaming channel. Each connector is a one-way replicator.

Connectors share a NATS connection and an optional connection to the NATS streaming server.

Request-reply is not supported.

The replicator is [configured with a NATS server-like format](docs/config.md), in a single file and uses the NATS logger.

An [optional HTTP/HTTPS endpoint](docs/monitoring.md) can be used for monitoring.

## Todo

* Integrate with coveralls

## Documentation

* [Build & Run the Replicator](docs/buildandrun.md)
* [Configuration](docs/config.md)
* [Monitoring](docs/monitoring.md)

## External Resources

* [NATS](https://nats.io/documentation/)
* [NATS server](https://github.com/nats-io/nats-server)
* [NATS Streaming](https://github.com/nats-io/nats-streaming-server)

[License-Url]: https://www.apache.org/licenses/LICENSE-2.0
[License-Image]: https://img.shields.io/badge/License-Apache2-blue.svg
[Build-Status-Url]: https://travis-ci.org/nats-io/nats-replicator
[Build-Status-Image]: https://travis-ci.org/nats-io/nats-replicator.svg?branch=master
[ReportCard-Url]: https://goreportcard.com/report/nats-io/nats-replicator
[ReportCard-Image]: https://goreportcard.com/badge/github.com/nats-io/nats-replicator?v=2

<a name="license"></a>

## License

Unless otherwise noted, the nats-replicator source files are distributed under the Apache Version 2.0 license found in the LICENSE file.
