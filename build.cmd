@echo off

set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -o build\linux_amd64\ cmd\pzrpc\pzrpc.go
go build -o build\linux_amd64\ cmd\pzrps\pzrps.go

SET CGO_ENABLED=0
SET GOOS=windows
SET GOARCH=amd64
go build -o build\windows_amd64\ cmd\pzrpc\pzrpc.go
go build -o build\windows_amd64\ cmd\pzrps\pzrps.go


SET CGO_ENABLED=0
SET GOOS=darwin
SET GOARCH=amd64
go build -o build\darwin_amd64\ cmd\pzrpc\pzrpc.go
go build -o build\darwin_amd64\ cmd\pzrps\pzrps.go
