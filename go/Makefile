.PHONY: build server client clean install deps

# 构建所有二进制文件
build: build-server build-client

# 构建服务器
build-server:
	cd cmd/server && go build -o ../../bin/tunnel-server .

# 构建客户端
build-client:
	cd cmd/client && go build -o ../../bin/tunnel-client .

# 安装依赖
deps:
	go mod tidy
	go mod download

# 运行服务器
server:
	go run cmd/server/main.go start -c server.yaml

# 运行客户端
client:
	go run cmd/client/main.go run -c client.yaml

# 生成令牌
token:
	go run cmd/server/main.go token add "new-client"

# 列出令牌
list-tokens:
	go run cmd/server/main.go token list -c server.yaml

# 创建目录
bin:
	mkdir -p bin

# 清理
clean:
	rm -rf bin/

# 交叉编译（Linux版本，用于VPS）
build-linux: bin
	cd cmd/server && GOOS=linux GOARCH=amd64 go build -o ../../bin/tunnel-server-linux .
	cd cmd/client && GOOS=linux GOARCH=amd64 go build -o ../../bin/tunnel-client-linux .

# 交叉编译（Windows版本）
build-windows: bin
	cd cmd/server && GOOS=windows GOARCH=amd64 go build -o ../../bin/tunnel-server.exe .
	cd cmd/client && GOOS=windows GOARCH=amd64 go build -o ../../bin/tunnel-client.exe .