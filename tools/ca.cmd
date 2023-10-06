@echo off

rmdir /Q/S ca>nul 2>nul
md ca\keys
md ca\certs
openssl genrsa -out ca/keys/ca.key 2048
openssl req -new -key ca/keys/ca.key -out ca/certs/ca.csr -subj "/C=CN/O=pzrp/OU=ca/CN=ca.pzrp.org"
openssl x509 -req -days 3650 -sha256 -extensions v3_ca -signkey ca/keys/ca.key -in ca/certs/ca.csr -out ca/certs/ca.crt
