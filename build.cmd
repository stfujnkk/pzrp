@echo off

for /F %%i in ('git describe --tags') do ( set version=%%i)
echo %version%

set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64

go build -trimpath -ldflags "-s -w -X 'main.VERSION=%version%'" -o bin\linux_amd64\ cmd\pzrpc\pzrpc.go
go build -trimpath -ldflags "-s -w -X 'main.VERSION=%version%'" -o bin\linux_amd64\ cmd\pzrps\pzrps.go

set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
go build -trimpath -ldflags "-s -w -X 'main.VERSION=%version%'" -o bin\windows_amd64\ cmd\pzrpc\pzrpc.go
go build -trimpath -ldflags "-s -w -X 'main.VERSION=%version%'" -o bin\windows_amd64\ cmd\pzrps\pzrps.go


set CGO_ENABLED=0
set GOOS=darwin
set GOARCH=amd64
go build -trimpath -ldflags "-s -w -X 'main.VERSION=%version%'" -o bin\darwin_amd64\ cmd\pzrpc\pzrpc.go
go build -trimpath -ldflags "-s -w -X 'main.VERSION=%version%'" -o bin\darwin_amd64\ cmd\pzrps\pzrps.go
