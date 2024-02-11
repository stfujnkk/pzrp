@echo off

rmdir /S ca
md ca\keys
md ca\certs
openssl genrsa -out ca/keys/ca.key 4096
openssl req -new -key ca/keys/ca.key -out ca/certs/ca.csr -subj "/C=CN/O=pzrp/OU=ca/CN=ca.pzrp.org"
echo subjectKeyIdentifier=hash > ca/certs/ca_cert_extensions
echo authorityKeyIdentifier=keyid:always,issuer >> ca/certs/ca_cert_extensions
echo basicConstraints=critical,CA:true >> ca/certs/ca_cert_extensions
openssl x509 -req -days 3650 -sha256 -extfile ca/certs/ca_cert_extensions -signkey ca/keys/ca.key -in ca/certs/ca.csr -out ca/certs/ca.crt
