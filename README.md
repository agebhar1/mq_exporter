# MQ metrics for Prometheus

[![CI](https://github.com/agebhar1/mq_exporter/actions/workflows/push.yml/badge.svg)](https://github.com/agebhar1/mq_exporter/actions/workflows/push.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/agebhar1/mq_exporter)](https://goreportcard.com/report/github.com/agebhar1/mq_exporter)

Prometheus exporter for [IBM MQ](https://www.ibm.com/products/mq).

The container image is available on Red Hat's [quay.io](https://quay.io/repository/agebhar1/mq_exporter) container registry. See [Container](#container) section for more details.

# Description

This exporter is for use in a restricted environment where it's no possible to run [Programmable command formats (PCFs)](https://www.ibm.com/docs/en/ibm-mq/9.2?topic=reference-programmable-command-formats-pcfs) and no other services exists such as [REST](https://www.ibm.com/docs/en/ibm-mq/9.2?topic=mq-messaging-using-rest-api).

To use the exporter only the `inquire` permission for the given queues are required to execute [MQINQ](https://www.ibm.com/docs/en/ibm-mq/9.2?topic=calls-mqinq-inquire-object-attributes).

More or less only the metrics of current queue depth `MQIA_CURRENT_Q_DEPTH` and maximum queue size `MQIA_MAX_Q_DEPTH` are (currently) supported as effect of not using PCFs. To run the exporter you need the IB MQ client library for C. The exporter itself is written in [Go](https://go.dev/) and uses IBMs [mq-golang](https://github.com/ibm-messaging/mq-golang) library, a Go binding to the C client library.

## Metrics

| Metric                              | Type  | [MQINQ attribute selector](https://www.ibm.com/docs/en/ibm-mq/9.2?topic=calls-mqinq-inquire-object-attributes) | Description                                                     |
|-------------------------------------|-------|----------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------|
| `mq_queue_current_depth`            | gauge | MQIA_CURRENT_Q_DEPTH                                                                                           | Number of messages on queue                                     |
| `mq_queue_max_depth`                | gauge | MQIA_MAX_Q_DEPTH                                                                                               | Maximum number of messages allowed on queue                     |
| `mq_queue_open_input_count`         | gauge | MQIA_OPEN_INPUT_COUNT                                                                                          | Number of `MQOPEN` calls that have the queue open for input     |
| `mq_queue_open_output_count`        | gauge | MQIA_OPEN_OUTPUT_COUNT                                                                                         | Number of `MQOPEN` calls that have the queue open               |
| `mq_queue_request_duration_seconds` | gauge | -                                                                                                              | Response time of `MQINQ` in seconds                             |
| `mq_queue_up`                       | gauge | -                                                                                                              | `1` if `MQINQ` was successful and within timeout, `0` otherwise |

Each metric contains the labels `channel`, `connection`, (queue) `name` and `queue_manager`.

Beside the above metrics, metrics for Go runtime is also provided by Prometheus go client collector and build info `mq_exporter_build_info`.

## Links

- [Authorizations for PCF commands](https://www.ibm.com/docs/en/ibm-mq/9.2?topic=windows-authorizations-pcf-commands)
- [Get started with the IBM MQ messaging REST API](https://developer.ibm.com/tutorials/mq-develop-mq-rest-api/)

## Alternatives/Products

- https://github.com/Appdynamics/websphere-mq-monitoring-extension
- https://github.com/Cinimex/mq-java-exporter
- https://github.com/ibm-messaging/mq-metric-samples

# Usage

# Configuration

```
usage: mq_exporter --config=CONFIG [<flags>]

A Prometheus exporter for MQ metrics.

Flags:
  -h, --help                Show context-sensitive help (also try --help-long and --help-man).
      --config=CONFIG       Path to config yaml file for MQ connections.
      --web.systemd-socket  Use systemd socket activation listeners instead of port listeners (Linux only).
      --web.listen-address=:9873 ...
                            Addresses on which to expose metrics and web interface. Repeatable for multiple addresses.
      --web.config.file=""  [EXPERIMENTAL] Path to configuration file that can enable TLS or authentication.
      --web.telemetry-path="/metrics"  
                            Path under which to expose metrics.
  -v, --version             Show application version.
      --log.level=info      Only log messages with the given severity or above. One of: [debug, info, warn, error]
      --log.format=logfmt   Output format of log messages. One of: [logfmt, json]
```

## Queue configuration

The queue configuration file is passed by `--config` and is required. It's a YAML with these attributes:

| Attribute         | Required | Description                                                                                                     |
|-------------------|:--------:|-----------------------------------------------------------------------------------------------------------------|
| `queueManager`    |    ✓     | name of the queue manager                                                                                       |
| `user` †          |          | username for channel authentication                                                                             |
| `password` †      |          | password for channel authentication                                                                             |
| `connName `       |    ✓     | host and port of MQ server                                                                                      |
| `channel `        |    ✓     | channel to connect to queues                                                                                    |
| `sslCipherSpec` ‡ |          | [Cipher Spec](https://www.ibm.com/docs/en/ibm-mq/9.2?topic=fields-sslcipherspec-mqchar32) which is used for TLS |
| `keyRepository` ‡ |          | location of [key repository](https://www.ibm.com/docs/en/ibm-mq/9.2?topic=mqsco-keyrepository-mqchar256)        |
| `timeout`         |          | timeout to inquire **all** queue metrics                                                                        |
| `queues`          |          | (string) list of (full) queue names                                                                             |

† if `user` is provided, then `password` is required and will be used; if `user` is absent then authentication will not be used <br>
‡ if `sslCipherSpec` is provided, then `keyRepository` is required and will be used; `sslCipherSpec` is absent TLS will not be used for MQ connection

An example for IBMs provided Container `icr.io/ibm-messaging/mq:latest` with the default [developer config](https://github.com/ibm-messaging/mq-container/blob/master/docs/developer-config.md) is:
```yaml
---
queueManager: QM1
user: admin
password: passw0rd
connName: localhost(1414)
channel: DEV.ADMIN.SVRCONN
queues:
  - DEV.QUEUE.1
  - DEV.QUEUE.2
  - DEV.QUEUE.3
```

An example with IBM MQ [encrypted connection ](https://developer.ibm.com/tutorials/mq-secure-msgs-tls/):
```yaml
---
queueManager: QM1
user: app
password: passw0rd
connName: localhost(1414)
channel: DEV.APP.SVRCONN
sslCipherSpec: TLS_RSA_WITH_AES_256_CBC_SHA256
keyRepository: keys/clientkey
queues:
  - DEV.QUEUE.1
  - DEV.QUEUE.2
  - DEV.QUEUE.3
```

Run with:
```shell
$ mkdir keys && pushd $_
$ openssl req -newkey rsa:2048 -nodes -keyout key.key -x509 -days 365 -out key.crt
$ chmod g+rw key.key # otherwise it can't be read w/ podman
$ # openssl x509 -text -noout -in key.crt
$ runmqakm -keydb -create -db clientkey.kdb -pw [!!pick_a_passw0rd_here!!] -type pkcs12 -stash
$ runmqakm -cert -add -label QM1.cert -db clientkey.kdb -stashed -trust enable -file key.crt
$ popd
$ podman run --rm \
    --env LICENSE=accept \
    --env MQ_APP_PASSWORD=passw0rd \
    --env MQ_QMGR_NAME=QM1 \
    --publish 1414:1414 \
    --volume $(pwd)/keys:/etc/mqm/pki/keys/mykey \
    icr.io/ibm-messaging/mq
```

## TLS and basic authentication

The MQ exporter uses Prometheus [exporter-toolkit](https://github.com/prometheus/exporter-toolkit) to support TLS and/or basic authentication. You need to pass a configuration file using the `--web.config` parameter.  The file format is described on [web configuration](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md).

# Build

To build the project the IBM client library is necessary due to the use of the `mq-golang` package. For development see [IBM MQ Downloads for developers](https://developer.ibm.com/articles/mq-downloads/) and choose 'Redist (grab & go) MQ Downloads'. For the sake of simplicity assume the library is located at `/opt/mqm` otherwise you have to update `CGO_CFLAGS` and `CGO_LDFLAGS` appropriate. For more details see [using the [mq-golang] package](https://github.com/ibm-messaging/mq-golang#using-the-package).

The project provides a `Makefile`. To run all tests and build the exporter call:

```shell
$ make
```

## Binary

The binary of the `mq_exporter` will not be available since it's build with C GO and depends on the build time versions of underlying libraries beside of IBM MQ itself.

## Container

The container image is available on Red Hat's [quay.io](https://quay.io/repository/agebhar1/mq_exporter) container registry and does no contain the IBM MQ libraries because of the missing unawareness about the legal guidelines. The containers user is 'nobody' (UID: 65534).

To run the container there are at least two options. Either build your own image like:
```dockerfile
FROM quay.io/agebhar1/mq_exporter:<tag>

COPY <mqm> /opt/mqm

…
```
Or by mounting the library on start like:
```shell
$ podman run --rm -v $(pwd)/mqm:/opt/mqm -v $(pwd)/config.yml:/etc/mq_exporter/config.yml -p 9873:9873 quay.io/agebhar1/mq_exporter:latest
…
```

The configuration file is expect to be `/etc/mq_exporter/config.yml` which can be mounted too.

## Links

- https://github.com/ibm-messaging/mq-golang
- https://developer.ibm.com/articles/mq-downloads
- https://developer.ibm.com/tutorials/mq-connect-app-queue-manager-containers/

# License

The repository and all contributions are licensed under
[APACHE 2.0](https://www.apache.org/licenses/LICENSE-2.0). Please review our [LICENSE](LICENSE) file.
