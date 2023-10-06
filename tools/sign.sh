#!/bin/sh

if [ "$1" = "" ];then
  echo certificate name needs to be specified
  exit
fi

rm -rf $1/*
mkdir -p $1/certs
mkdir -p $1/keys
openssl genrsa -out $1/keys/$1.key 2048
openssl req -new -key $1/keys/$1.key -out $1/certs/$1.csr -subj "/C=CN/O=pzrp/OU=$1/CN=$1.pzrp.org"
openssl x509 -req -days 365 -sha256 -extensions v3_req -CA ca/certs/ca.crt -CAkey ca/keys/ca.key -in $1/certs/$1.csr -out $1/certs/$1.crt -CAcreateserial
