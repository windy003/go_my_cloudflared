# Docker 部署指南

本项目已支持Docker部署，提供了完整的容器化解决方案。

## 快速开始

### 1. 构建并启动服务

```bash
# 构建并启动容器
docker-compose up -d

# 查看日志
docker-compose logs -f tunnel-server

# 查看服务状态
docker-compose ps
```

### 2. 停止服务

```bash
# 停止服务
docker-compose down

# 停止服务并删除数据卷
docker-compose down -v
```

## 配置说明

### 端口映射

- `6000`: HTTP服务端口
- `6001`: WebSocket端口  
- `6443`: HTTPS端口 (可选)
- `6444`: WebSocket Secure端口 (可选)

### 目录挂载

- `./config`: 配置文件目录
- `./certs`: SSL证书目录 (如果启用HTTPS)
- `./logs`: 日志文件目录

### 配置文件

编辑 `config/server.yml` 来自定义服务器配置：

```yaml
server:
  httpPort: 6000
  wsPort: 6001
  host: "0.0.0.0"
  publicDomain: "your-domain.com"  # 修改为你的域名
  
auth:
  requireAuth: true
  tokens:
    - "your-secure-token"  # 修改为安全的令牌
```

## HTTPS配置

如果需要启用HTTPS，请：

1. 将SSL证书放在 `certs` 目录下
2. 修改 `config/server.yml`:

```yaml
server:
  enableHttps: true
  httpsPort: 6443
  certFile: "/app/certs/server.crt"
  keyFile: "/app/certs/server.key"
  enableWss: true
  wssPort: 6444
```

## 监控和维护

### 健康检查

容器内置了健康检查，访问健康检查端点：

```bash
curl http://localhost:6000/health
```

### 查看客户端连接

```bash
curl http://localhost:6000/clients
```

### 容器资源监控

```bash
# 查看容器资源使用情况
docker stats go-tunnel-server

# 查看容器详细信息
docker inspect go-tunnel-server
```

## 生产环境部署建议

1. **使用外部配置管理**：
   - 使用Docker secrets或环境变量管理敏感配置
   - 不要在镜像中包含生产配置

2. **资源限制**：
   - 根据实际需求调整 `docker-compose.yml` 中的资源限制

3. **日志管理**：
   - 配置日志轮转和持久化存储
   - 考虑使用ELK或其他日志管理系统

4. **网络安全**：
   - 使用防火墙限制端口访问
   - 启用HTTPS和强认证

5. **备份和恢复**：
   - 定期备份配置文件和证书
   - 制定灾难恢复计划

## 故障排除

### 常见问题

1. **端口冲突**：
   ```bash
   # 检查端口占用
   netstat -tulpn | grep :6000
   ```

2. **权限问题**：
   ```bash
   # 检查目录权限
   ls -la config/ certs/ logs/
   ```

3. **容器启动失败**：
   ```bash
   # 查看详细错误信息
   docker-compose logs tunnel-server
   ```

### 调试模式

如需调试，可以临时运行容器：

```bash
# 交互式运行容器
docker run -it --rm -p 6000:6000 -p 6001:6001 \
  -v $(pwd)/config:/app/config \
  go-tunnel-server /bin/sh
```

## 更新和升级

```bash
# 重新构建并启动
docker-compose build --no-cache
docker-compose up -d

# 或者拉取最新镜像（如果使用远程镜像）
docker-compose pull
docker-compose up -d
```