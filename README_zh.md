# pzrp

[README](README.md) | [中文文档](README_zh.md)

一个轻量的反向代理，可以帮助您将 NAT 或防火墙后面的本地服务器暴露在互联网上。

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

你可以在配置文件里指定证书来进行加密。证书可以通过 openssl 生成或使用 tools 文件夹里的脚本生成。

其中`ca_cert`指定 CA 证书的路径。`cert_file`、`key_file`分别指定证书和密钥路径。

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

另外可以通过设置token来拒绝简单地鉴权。只需在`pzrps.json`加上token字段。`pzrps.json`同理。

```json
{
  "bind_addr": "0.0.0.0",
  "bind_port": 8848,
  "token": "你的密码"
}
```

## 性能

在每秒 1000 个请求的情况下,可以达到 10964 的吞吐量。具体数据参考[压力测试报告](stress_test.zip)。

延迟约25ms服务器的测试结果如下：
```txt
udp (direct) mean: 23.9206, median: 23.895, variance: 1.4923329696969698, max: 27.44, min: 21.53
udp (pzrp) mean: 48.2616, median: 48.21, variance: 1.9760782222222222, max: 53.93, min: 45.65
tcp (direct) mean: 24.3326, median: 24.25, variance: 1.8366537777777778, max: 27.5, min: 21.9
tcp (pzrp) mean: 48.7418, median: 48.675, variance: 1.6446633939393935, max: 51.54, min: 45.87
```
