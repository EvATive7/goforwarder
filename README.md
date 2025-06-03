# goforwarder

**goforwarder** 是一个用 Go 编写的轻量级 HTTP 反向代理服务器，通过自定义配置文件将多个外部站点映射到本地域名进行访问，同时支持响应内容的域名替换。

## ✨ 特性

* 基于域名的反向代理映射
* 自动重写响应中的 URL（如 HTML、JSON、XML）
* 支持 alias 输出为 HTTPS 链接（可选）
* 支持通过上游代理转发请求（支持 `http://` 和 `https://` 代理）
* 配置简单，基于 YAML 文件

## 📁 示例配置（`data/config.yml`）

详见 [`type Config struct`](src/main.go#L17)

```yaml
host_rules:
  - origin: example.com
    alias: mirror.localhost
  - origin: another.com
    alias: test.localhost

alias_uses_https: false

settings:
  proxy: http://127.0.0.1:7890
  address: 127.0.0.1:8080
```

字段说明：

* `host_rules`：定义原始站点与本地 alias 的映射关系
* `alias_uses_https`：是否将 alias 替换为 `https` 协议（默认用于响应内容替换）
* `settings.proxy`：可选，设置上游代理服务器（支持 `http://` 或 `https://`）
* `settings.address`：服务监听地址和端口

## 🚀 快速开始

### 1. 下载或构建可执行文件

构建（可选）：

```bash
cd src
go build
```

或直接使用已提供的 `goforwarder.exe` 文件。

### 2. 配置 YAML

编辑 `data/config.yml`，设置你需要的代理规则。

### 3. 启动服务

```bash
goforwarder.exe
```

程序启动后将监听配置中的 `address`，并根据你访问的 alias 自动转发请求。

## 🛠️ TODO

* 添加高级功能
* 增加 CLI 参数支持
* Web UI

## ⚠️ 注意

 本项目仅用于学习和测试用途，请勿用于违反法律法规的行为。
