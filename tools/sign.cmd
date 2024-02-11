@echo off

if "%~1" == "" goto end1
set ip=127.0.0.1
if "%~2" neq "" set ip=%~2

rmdir /S %1
md %1\keys
md %1\certs
openssl genrsa -out %1/keys/%1.key 2048
openssl req -new -key %1/keys/%1.key -out %1/certs/%1.csr -subj "/C=CN/O=pzrp/OU=%1/CN=%1.pzrp.org"
set extfile=%1/certs/%1_cert_extensions
echo basicConstraints=CA:FALSE > %extfile%
echo keyUsage=nonRepudiation, digitalSignature, keyEncipherment >> %extfile%
echo subjectAltName=DNS.1:localhost, IP.1:%ip% >> %extfile%
openssl x509 -req -days 365 -sha256 -extfile %extfile% -CA ca/certs/ca.crt -CAkey ca/keys/ca.key -in %1/certs/%1.csr -out %1/certs/%1.crt -CAcreateserial
copy /y "ca\certs\ca.crt" "%1\ca.crt"
goto:eof

:end1
echo certificate name needs to be specified
goto:eof
