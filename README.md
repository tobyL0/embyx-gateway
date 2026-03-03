# emby-gateway

一个简单的 Emby 反向代理网关，监听 `8096` 端口并转发请求到 Emby 上游服务。

## 功能

- 默认监听 `:8096`
- 远端 Emby 地址可配置
- 默认上游地址：`http://127.0.0.1:18096`
- 将所有用户请求转发到上游 Emby
- 自动补充常见代理头：
  - `X-Forwarded-For`
  - `X-Forwarded-Proto`
  - `X-Forwarded-Host`
- 提供健康检查：`/healthz`
- 支持优雅停机（SIGINT/SIGTERM）

## 运行

```bash
go run .
```

指定上游地址：

```bash
go run . -upstream http://127.0.0.1:18096
```

或通过环境变量：

```bash
UPSTREAM_URL=http://127.0.0.1:18096 LISTEN_ADDR=:8096 go run .
```

## 参数

- `-listen`：网关监听地址，默认 `:8096`
- `-upstream`：Emby 上游地址，默认 `http://127.0.0.1:18096`

## 健康检查

```bash
curl http://127.0.0.1:8096/healthz
```
