# pzrp
[README](README.md) | [中文文档](README_zh.md)

A lightweight reverse proxy to help you expose a local server behind a NAT or firewall to the internet.
## Compile

Execute the following command

- windows

  ```bat
  .\build.cmd
  ```

- linux/mac

  ```bash
  ./build.sh
  ```

## Get started!

- Writing Configuration Files

  client configuration (pzrpc.json)

  ```json
  {
    "server_addr": "127.0.0.1",
    "server_port": 8848,
    "services": {
      "s0": {
        "type": "udp",
        "local_ip": "127.0.0.1",
        "local_port": "8200-8205",
        "remote_port": "9200-9205"
      },
      "s1": {
        "type": "tcp",
        "local_ip": "127.0.0.1",
        "local_port": "8888",
        "remote_port": "8000"
      }
    }
  }
  ```

  Server Configuration (pzrps.json)
  ```json
  {
    "bind_addr": "0.0.0.0",
    "bind_port": 8848
  }
  ```

- Run

  Start the server using the following command: `./pzrps -config ./pzrps.json`。

  Start the client using the following command:`./pzrpc -config ./pzrpc.json`。

  If you need to run in the background for a long time, it is recommended to combine other tools, such as `systemd` or `supervisor`。

  If you are a Windows user, you need to execute the same command from the cmd.


## Secure

You can specify a certificate in the configuration file for encryption. Certificates can be generated through openssl or using scripts in the tools folder.

The `ca_cert` field specifies the path of the CA certificate.
The `cert_file`,`key_file` field specifies the certificate and key paths respectively.

  client configuration (pzrpc.json)
  ```json
  {
    "server_addr": "127.0.0.1",
    "server_port": 8848,
    "services": {
      "s0": {
        "type": "udp",
        "local_ip": "127.0.0.1",
        "local_port": "8200-8205",
        "remote_port": "9200-9205"
      },
      "s1": {
        "type": "tcp",
        "local_ip": "127.0.0.1",
        "local_port": "8888",
        "remote_port": "8000"
      }
    },
    "cert_file": "client/certs/client.crt",
    "key_file": "client/keys/client.key",
    "ca_cert": "ca/certs/ca.crt"
  }
  ```

  Server Configuration (pzrps.json)
  ```json
  {
    "bind_addr": "0.0.0.0",
    "bind_port": 8848,
    "cert_file": "server/certs/server.crt",
    "key_file": "server/keys/server.key",
    "ca_cert": "ca/certs/ca.crt"
  }
  ```
## Performance

At a rate of 1000 requests per second, a throughput of 10964 can be achieved. Please refer to the [stress test report](stress_test.zip) for specific data