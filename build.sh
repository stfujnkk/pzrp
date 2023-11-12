#!/bin/sh

CGO_ENABLED=0
GOOS=linux
GOARCH=amd64
go build -ldflags "-s -w" -o bin/linux_amd64/ cmd/pzrpc/pzrpc.go
go build -ldflags "-s -w" -o bin/linux_amd64/ cmd/pzrps/pzrps.go

CGO_ENABLED=0
GOOS=windows
GOARCH=amd64
go build -ldflags "-s -w" -o bin/windows_amd64/ cmd/pzrpc/pzrpc.go
go build -ldflags "-s -w" -o bin/windows_amd64/ cmd/pzrps/pzrps.go

CGO_ENABLED=0
GOOS=darwin
GOARCH=amd64
go build -ldflags "-s -w" -o bin/darwin_amd64/ cmd/pzrpc/pzrpc.go
go build -ldflags "-s -w" -o bin/darwin_amd64/ cmd/pzrps/pzrps.go
