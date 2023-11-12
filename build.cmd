@echo off

set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-s -w" -o bin\linux_amd64\ cmd\pzrpc\pzrpc.go
go build -ldflags "-s -w" -o bin\linux_amd64\ cmd\pzrps\pzrps.go

set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-s -w" -o bin\windows_amd64\ cmd\pzrpc\pzrpc.go
go build -ldflags "-s -w" -o bin\windows_amd64\ cmd\pzrps\pzrps.go


set CGO_ENABLED=0
set GOOS=darwin
set GOARCH=amd64
go build -ldflags "-s -w" -o bin\darwin_amd64\ cmd\pzrpc\pzrpc.go
go build -ldflags "-s -w" -o bin\darwin_amd64\ cmd\pzrps\pzrps.go
