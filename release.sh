#!/bin/sh
CGO_ENABLED=0 go build -buildvcs=false -ldflags "-s -w -X main.version=v5.3.1-$(git describe --tags --long --always)-geosite" -trimpath -o mosdns