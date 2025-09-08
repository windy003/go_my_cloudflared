# Go Tunnel - 高性能内网穿透工具

用 Go 语言实现的类似 Cloudflare Tunnel 的内网穿透工具，性能更优，配置更灵活。

## 🎯 特性

- ✅ **高性能**: Go 语言实现，并发性能优秀
- ✅ **配置灵活**: 支持 YAML/JSON 配置文件和命令行参数
- ✅ **自动重连**: 网络断开时自动重连
- ✅ **令牌认证**: 安全的令牌认证机制
- ✅ **HTTPS支持**: 支持TLS/SSL加密传输，WSS安全WebSocket
- ✅ **交叉编译**: 支持 Linux/Windows/macOS 多平台

## 🚀 快速开始

### 方式1: 直接运行（推荐开发使用）

#### 1. 安装依赖
```bash
cd go
go mod tidy
```

#### 2. VPS 服务器部署

上传 Go 代码到 VPS：
```bash
# 上传整个go文件夹到VPS
scp -r go/ user@windy.run:~/tunnel-go/
```

在 VPS 上直接运行服务器：
```bash
ssh user@windy.run
cd tunnel-go

# 安装依赖（VPS上需要Go环境）
go mod tidy

# 直接运行服务器
go run cmd/server/main.go start -c server.yaml

# 或使用命令行参数
go run cmd/server/main.go start --http-port 6000 --ws-port 6001 --host 0.0.0.0
```

#### 3. 内网 PC 客户端

```bash
cd go

# 创建配置文件
go run cmd/client/main.go config init

# 编辑 tunnel.json，然后运行客户端
go run cmd/client/main.go run -c tunnel.json

# 或使用命令行参数
go run cmd/client/main.go run --tunnel-url ws://windy.run:6001 --auth-token your-token --local-port 3000
```

### 方式2: 编译后运行（推荐生产使用）

#### 1. 编译Linux版本
```bash
make build-linux
```

#### 2. 上传到VPS
```bash
# 上传二进制文件
scp bin/tunnel-server-linux user@windy.run:/usr/local/bin/tunnel-server
scp server.yaml user@windy.run:~/server.yaml

# 设置执行权限
ssh user@windy.run
chmod +x /usr/local/bin/tunnel-server
```

#### 3. 启动服务器
```bash
# 使用配置文件启动
tunnel-server start -c server.yaml

# 或使用命令行参数
tunnel-server start --http-port 6000 --ws-port 6001 --host 0.0.0.0
```

你会看到：
```
启动隧道服务器...
HTTP 端口: 6000
WebSocket 端口: 6001
认证令牌数量: 2
HTTP服务器启动在端口 6000
WebSocket服务器启动在端口 6001
管理接口: http://localhost:6000/health
客户端列表: http://localhost:6000/clients
```

#### 开放端口
```bash
sudo ufw allow 6000
sudo ufw allow 6001
```

### 3. 内网 PC 客户端

#### 创建配置文件
```bash
# Windows
go\tunnel-client.exe config init

# Linux/macOS  
./bin/tunnel-client config init
```

#### 编辑配置
编辑生成的 `tunnel.json`：
```json
{
  "tunnel": {
    "url": "ws://windy.run:6001",
    "authToken": "my-secure-token-12345"
  },
  "local": {
    "host": "localhost",
    "port": 3000
  }
}
```

#### 启动客户端
```bash
# Windows
go\tunnel-client.exe run -c tunnel.json

# Linux/macOS
./bin/tunnel-client run -c tunnel.json

# 或使用命令行参数
./bin/tunnel-client run --tunnel-url ws://windy.run:6001 --auth-token my-secure-token-12345 --local-port 3000
```

### 4. 测试连接

现在访问 `http://windy.run:6000` 就能看到你内网的服务了！

## 🔐 HTTPS 配置

### 1. 生成SSL证书

#### 自签名证书（测试用）
```bash
# Linux/macOS
chmod +x generate-cert.sh
./generate-cert.sh

# Windows
generate-cert.bat
```

#### 正式SSL证书
将正式的SSL证书文件放在Go目录下，并在配置文件中指定路径。

### 2. 启用HTTPS服务器

#### 使用配置文件
```bash
# 编辑配置文件 server-https.yaml
go run cmd/server/main.go start -c server-https.yaml
```

#### 使用命令行参数
```bash
# 只启用HTTPS
go run cmd/server/main.go start \
  --enable-https \
  --https-port 6443 \
  --cert-file server.crt \
  --key-file server.key

# 同时启用HTTP和HTTPS
go run cmd/server/main.go start \
  --http-port 6000 \
  --https-port 6443 \
  --enable-https \
  --cert-file server.crt \
  --key-file server.key

# 启用WSS (安全WebSocket)
go run cmd/server/main.go start \
  --enable-https \
  --enable-wss \
  --https-port 6443 \
  --wss-port 6444 \
  --cert-file server.crt \
  --key-file server.key
```

### 3. 客户端连接WSS

#### 使用配置文件（推荐）
```bash
# 1. 创建WSS配置文件
go run cmd/client/main.go config init

# 2. 编辑tunnel.json，修改为WSS配置：
# {
#   "tunnel": {
#     "url": "wss://windy.run:6444",
#     "authToken": "your-token",
#     "insecureSkipVerify": true,
#     "serverName": "windy.run"
#   }
# }

# 3. 启动客户端
go run cmd/client/main.go run -c tunnel.json
```

#### 使用命令行参数
```bash
# 注意：命令行方式无法配置SSL选项，仅适用于有效SSL证书
go run cmd/client/main.go run \
  --tunnel-url wss://windy.run:6444 \
  --auth-token your-token \
  --local-port 3000
```

#### 自签名证书配置
对于自签名证书，必须在配置文件中设置：
```json
{
  "tunnel": {
    "url": "wss://windy.run:6444",
    "authToken": "your-token",
    "insecureSkipVerify": true,
    "serverName": "windy.run"
  }
}
```

### 4. 访问HTTPS服务
```bash
# 访问HTTPS端点
curl -k https://windy.run:6443

# 访问HTTPS管理接口
curl -k https://windy.run:6443/health
curl -k https://windy.run:6443/clients
```

### 5. 防火墙配置
```bash
# 开放HTTPS和WSS端口
sudo ufw allow 6443/tcp  # HTTPS
sudo ufw allow 6444/tcp  # WSS
```

## 💡 完整使用示例（go run方式）

### VPS 服务器操作

```bash
# 1. 上传代码到VPS
scp -r go/ user@windy.run:~/tunnel-go/

# 2. SSH到VPS
ssh user@windy.run
cd tunnel-go

# 3. 安装Go依赖
go mod tidy

# 4. 生成认证令牌
go run cmd/server/main.go token add "my-pc"
# 输出: ✓ 新令牌已创建: my-pc - token_1699123456_my-pc

# 5. 编辑服务器配置（可选）
nano server.yaml

# 6. 启动服务器
go run cmd/server/main.go start --http-port 6000 --ws-port 6001 --host 0.0.0.0

# 7. 开放端口
sudo ufw allow 6000
sudo ufw allow 6001
```

### 内网 PC 操作

```bash
# 1. 进入Go目录
cd go

# 2. 安装依赖
go mod tidy

# 3. 创建客户端配置
go run cmd/client/main.go config init

# 4. 编辑配置文件
nano tunnel.json
# 修改为:
# {
#   "tunnel": {
#     "url": "ws://windy.run:6001",
#     "authToken": "token_1699123456_my-pc"
#   },
#   "local": {
#     "host": "localhost",
#     "port": 3000
#   }
# }

# 5. 启动本地Web服务（例如）
python -m http.server 3000

# 6. 启动隧道客户端
go run cmd/client/main.go run -c tunnel.json

# 或者直接用命令行参数
go run cmd/client/main.go run \
  --tunnel-url ws://windy.run:6001 \
  --auth-token token_1699123456_my-pc \
  --local-port 3000
```

### 验证连接

```bash
# 访问公网地址
curl http://windy.run:6000

# 查看服务器状态
curl http://windy.run:6000/health

# 查看客户端列表
curl http://windy.run:6000/clients
```

## 📋 命令说明

### 服务器命令

#### 直接运行方式
```bash
# 启动服务器
go run cmd/server/main.go start [flags]

# 令牌管理
go run cmd/server/main.go token add <name>     # 添加令牌
go run cmd/server/main.go token list          # 列出令牌

# 示例
go run cmd/server/main.go start -c server.yaml
go run cmd/server/main.go start --http-port 6000 --ws-port 6001
go run cmd/server/main.go token add "my-client"
go run cmd/server/main.go token list -c server.yaml
```

#### 编译后运行方式
```bash
# 启动服务器
tunnel-server start [flags]

# 令牌管理  
tunnel-server token add <name>          # 添加令牌
tunnel-server token list               # 列出令牌
```

#### 参数说明
```
--config, -c        配置文件路径
--http-port         HTTP端口 (默认6000)
--ws-port          WebSocket端口 (默认6001)
--host             监听地址 (默认0.0.0.0)
--enable-https     启用HTTPS服务器
--https-port       HTTPS端口 (默认6443)
--cert-file        SSL证书文件路径
--key-file         SSL私钥文件路径
--enable-wss       启用WebSocket Secure (WSS)
--wss-port         WSS端口 (默认6444)
```

### 客户端命令

#### 直接运行方式
```bash
# 启动客户端
go run cmd/client/main.go run [flags]

# 配置管理
go run cmd/client/main.go config init        # 创建配置文件
go run cmd/client/main.go config show        # 显示配置

# 示例
go run cmd/client/main.go run -c tunnel.json
go run cmd/client/main.go run --tunnel-url ws://windy.run:6001 --auth-token token123 --local-port 3000
go run cmd/client/main.go config init
```

#### 编译后运行方式
```bash
# 启动客户端
tunnel-client run [flags]

# 配置管理
tunnel-client config init             # 创建配置文件
tunnel-client config show            # 显示配置
```

#### 参数说明
```
--config, -c         配置文件路径
--tunnel-url         服务器地址
--auth-token         认证令牌
--local-host         本地主机 (默认localhost)
--local-port         本地端口 (默认3000)
```

## ⚙️ 配置文件

### 服务器配置 (server.yaml)

```yaml
server:
  httpPort: 6000              # HTTP服务端口
  wsPort: 6001               # WebSocket端口  
  host: "0.0.0.0"            # 监听地址
  publicDomain: "windy.run"   # 公网域名
  requestTimeout: 30000       # 请求超时(毫秒)
  maxClients: 100            # 最大客户端数
  
  # HTTPS 配置
  enableHttps: true          # 启用HTTPS
  httpsPort: 6443           # HTTPS端口
  certFile: "server.crt"    # SSL证书文件路径
  keyFile: "server.key"     # SSL私钥文件路径
  
  # WebSocket Secure 配置
  enableWss: true           # 启用WSS (WebSocket over TLS)
  wssPort: 6444            # WSS端口

auth:
  requireAuth: true
  tokens:
    - "token1"
    - "token2"
```

### 客户端配置 (client.yaml)

```yaml
tunnel:
  url: "ws://windy.run:6001"        # 服务器地址 (HTTP)
  # url: "wss://windy.run:6444"     # 或使用WSS安全连接
  authToken: "your-token"           # 认证令牌
  reconnectAttempts: 10             # 重连次数
  reconnectDelay: 5000             # 重连延迟
  
  # WSS/TLS 配置 (使用wss://时需要)
  insecureSkipVerify: true         # 跳过证书验证（自签名证书）
  serverName: "windy.run"          # 服务器名称
  caCertFile: ""                   # CA证书文件路径（可选）

local:
  host: "localhost"                # 本地服务地址
  port: 3000                      # 本地服务端口
```

## 🔧 开发和构建

### 直接运行（开发推荐）

```bash
# 进入项目目录
cd go

# 安装依赖
go mod tidy

# 运行服务器（终端1）
go run cmd/server/main.go start -c server.yaml

# 运行客户端（终端2）  
go run cmd/client/main.go run -c client.yaml

# 生成令牌
go run cmd/server/main.go token add "new-client"

# 查看令牌列表
go run cmd/server/main.go token list -c server.yaml

# 创建客户端配置
go run cmd/client/main.go config init

# 查看客户端配置
go run cmd/client/main.go config show -c tunnel.json
```

### 使用 Makefile（快捷方式）

```bash
# 安装依赖
make deps

# 运行服务器
make server

# 运行客户端
make client

# 生成令牌
make token
```

### 构建发布版本

```bash
# 构建当前平台
make build

# 构建Linux版本（用于VPS）
make build-linux

# 构建Windows版本
make build-windows

# 清理构建文件
make clean
```

## 📊 监控接口

服务器提供以下监控接口：

```bash
# 健康检查
curl http://windy.run:6000/health

# 客户端列表
curl http://windy.run:6000/clients

# 输出示例
{
  "status": "healthy",
  "clients": 1,
  "uptime": 3600
}
```

## 🔍 故障排除

### 1. 服务器启动失败

```bash
# 检查端口占用
netstat -tlnp | grep 6000

# 使用不同端口
tunnel-server start --http-port 8000 --ws-port 8001
```

### 2. 客户端连接失败

```bash
# 测试服务器连通性
telnet windy.run 6001

# 检查令牌是否正确
tunnel-server token list -c server.yaml
```

### 3. 防火墙问题

```bash
# 开放端口
sudo ufw allow 6000
sudo ufw allow 6001

# 检查云服务商安全组设置
```

## 🔒 安全建议

- 使用强随机令牌
- 定期更换认证令牌  
- ✅ 启用HTTPS (配置SSL证书)
- ✅ 使用WSS替代WS连接
- 限制客户端连接数
- 监控异常访问
- 生产环境使用正式SSL证书

## 📈 性能优势

相比 Node.js 版本：
- **启动更快**: 秒级启动
- **内存占用更少**: 通常 < 50MB
- **并发性能更好**: Go 的协程模型
- **部署简单**: 单个二进制文件
- **跨平台**: 无需安装运行时

## 🎯 使用场景

- **开发环境**: 展示本地开发项目
- **家庭服务器**: 外网访问NAS/路由器
- **IoT设备**: 远程管理内网设备  
- **游戏服务器**: 朋友联机游戏
- **临时服务**: 快速暴露本地服务

现在你有了一个高性能的 Go 版本隧道工具！🚀