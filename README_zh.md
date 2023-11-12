# pzrp
[README](README.md) | [中文文档](README_zh.md)

一个轻量的反向代理，可以帮助您将NAT或防火墙后面的本地服务器暴露在互联网上。

## 编译

执行如下命令

- windows

  ```bat
  .\build.cmd
  ```

- linux/mac

  ```bash
  ./build.sh
  ```

## 开始使用！

- 编写配置文件

  客户端配置（pzrpc.json）

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

  服务端配置（pzrps.json）
  ```json
  {
    "bind_addr": "0.0.0.0",
    "bind_port": 8848
  }
  ```

- 运行

  使用以下命令启动服务器：`./pzrps -config ./pzrps.json`。

  使用以下命令启动客户端：`./pzrpc -config ./pzrpc.json`。

  如果需要在后台长期运行，建议结合其他工具，如 `systemd` 或 `supervisor`。

  如果您是 Windows 用户，需要在命令提示符中执行相同的命令。


## 安全

你可以在配置文件里指定证书来进行加密。证书可以通过openssl生成或使用tools文件夹里的脚本生成。

其中`ca_cert`指定CA证书的路径。`cert_file`、`key_file`分别指定证书和密钥路径。

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

  服务端配置（pzrps.json）
  ```json
  {
    "bind_addr": "0.0.0.0",
    "bind_port": 8848,
    "cert_file": "server/certs/server.crt",
    "key_file": "server/keys/server.key",
    "ca_cert": "ca/certs/ca.crt"
  }
  ```

## 性能

在每秒1000个请求的情况下,可以达到10964的吞吐量。具体数据参考[压力测试报告](stress_test.zip)