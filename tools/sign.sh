#!/bin/sh

set -e
if [ "$1" = "" ]; then
  echo certificate name needs to be specified
  exit
fi
ip="127.0.0.1"
if [ "$2" != "" ]; then
  ip="$2"
fi

if [ -e "$1" ]; then
  read -p "Do you want to delete '$1'?" ok
  if [ "$ok" = "y" ]; then
    rm -rf $1/*
  else
    exit 1
  fi
fi
mkdir -p $1/certs
mkdir -p $1/keys
cp ca/certs/ca.crt $1/ca.crt
openssl genrsa -out $1/keys/$1.key 2048
openssl req -new -key $1/keys/$1.key -out $1/certs/$1.csr -subj "/C=CN/O=pzrp/OU=$1/CN=$1.pzrp.org"
extfile="$1/certs/${1}_cert_extensions"
echo "basicConstraints=CA:FALSE" >$extfile
echo "keyUsage=nonRepudiation, digitalSignature, keyEncipherment" >>$extfile
echo "subjectAltName=DNS.1:localhost, IP.1:$ip" >>$extfile
openssl x509 -req -days 365 -sha256 -extfile $extfile -CA ca/certs/ca.crt -CAkey ca/keys/ca.key -in $1/certs/$1.csr -out $1/certs/$1.crt -CAcreateserial
