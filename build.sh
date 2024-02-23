#!/bin/sh

version=$(git describe --tags)
echo $version

CGO_ENABLED=0
GOOS=linux
GOARCH=amd64
go build -trimpath -ldflags "-s -w -X 'main.VERSION=$version'" -o bin/linux_amd64/ cmd/pzrpc/pzrpc.go
go build -trimpath -ldflags "-s -w -X 'main.VERSION=$version'" -o bin/linux_amd64/ cmd/pzrps/pzrps.go

CGO_ENABLED=0
GOOS=windows
GOARCH=amd64
go build -trimpath -ldflags "-s -w -X 'main.VERSION=$version'" -o bin/windows_amd64/ cmd/pzrpc/pzrpc.go
go build -trimpath -ldflags "-s -w -X 'main.VERSION=$version'" -o bin/windows_amd64/ cmd/pzrps/pzrps.go

CGO_ENABLED=0
GOOS=darwin
GOARCH=amd64
go build -trimpath -ldflags "-s -w -X 'main.VERSION=$version'" -o bin/darwin_amd64/ cmd/pzrpc/pzrpc.go
go build -trimpath -ldflags "-s -w -X 'main.VERSION=$version'" -o bin/darwin_amd64/ cmd/pzrps/pzrps.go
