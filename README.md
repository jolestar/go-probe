Go-Probe
=====

[![Build Status](https://travis-ci.org/jolestar/go-probe.svg?branch=master)](https://travis-ci.org/jolestar/go-probe)

go-probe is a simple server environment probe.

## Usage

docker run --name go-probe -p 8080:80 -d jolestar/go-probe

1. open [http://localhost:8080](http://localhost:8080) by browser, will get html result.
2. curl -H "accept:application/yaml" http://localhost:8080
3. curl -H "accept:application/json" http://localhost:8080

## Support Probe Function

* Env: show system environment variable
* HostInfo: show host-info, such as: hostname, platform, kernel version
* CpuInfo
* NetworkInfo: network interfaces
* RequestInfo: request remote addr, headers.
* LoadAvg
* MemoryInfo