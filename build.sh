#!/bin/bash

set -e

VERSION=$(cat VERSION)

mkdir -p out

go build -o out/assismgr-linux-amd64 src/*.go 
GOARCH=arm64 go build -o out/assismgr-linux-arm64 src/* 

./scripts/build_deb.sh $VERSION LanSilence:642459901@qq.com