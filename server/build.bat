%echo off
set GOOS=linux
set GOARCH=amd64
go build -o server
set GOOS=windows
set GOARCH=amd64