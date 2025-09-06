package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Config 服务器配置
type Config struct {
	Server struct {
		HTTPPort      int    `yaml:"httpPort" json:"httpPort"`
		WSPort        int    `yaml:"wsPort" json:"wsPort"`
		Host          string `yaml:"host" json:"host"`
		PublicDomain  string `yaml:"publicDomain" json:"publicDomain"`
		RequestTimeout int   `yaml:"requestTimeout" json:"requestTimeout"`
		MaxClients    int    `yaml:"maxClients" json:"maxClients"`
	} `yaml:"server" json:"server"`
	Auth struct {
		RequireAuth bool     `yaml:"requireAuth" json:"requireAuth"`
		Tokens      []string `yaml:"tokens" json:"tokens"`
	} `yaml:"auth" json:"auth"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	config := &Config{}
	config.Server.HTTPPort = 6000
	config.Server.WSPort = 6001
	config.Server.Host = "0.0.0.0"
	config.Server.PublicDomain = ""
	config.Server.RequestTimeout = 30000
	config.Server.MaxClients = 100
	config.Auth.RequireAuth = true
	config.Auth.Tokens = []string{"default-token"}
	return config
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()
	
	if configPath == "" {
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, nil // 使用默认配置
	}

	// 尝试解析 YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		// 如果YAML失败，尝试JSON
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("配置文件格式错误: %v", err)
		}
	}

	return config, nil
}

// Client 客户端连接
type Client struct {
	ID       string
	Conn     *websocket.Conn
	Host     string
	Port     int
	LastPing time.Time
}

// TunnelServer 隧道服务器
type TunnelServer struct {
	config         *Config
	clients        map[string]*Client
	clientsMux     sync.RWMutex
	upgrader       websocket.Upgrader
	httpServer     *http.Server
	wsServer       *http.Server
	pendingRequests map[string]chan HTTPResponse
	requestMux     sync.RWMutex
}

// HTTPResponse HTTP响应结构
type HTTPResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error"`
}

// NewTunnelServer 创建隧道服务器
func NewTunnelServer(config *Config) *TunnelServer {
	return &TunnelServer{
		config:          config,
		clients:         make(map[string]*Client),
		pendingRequests: make(map[string]chan HTTPResponse),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许跨域
			},
		},
	}
}

// Start 启动服务器
func (s *TunnelServer) Start() error {
	// 启动WebSocket服务器
	go s.startWebSocketServer()
	
	// 启动HTTP服务器
	return s.startHTTPServer()
}

// startWebSocketServer 启动WebSocket服务器
func (s *TunnelServer) startWebSocketServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWebSocket)
	
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.WSPort)
	
	// 创建TCP4监听器，强制使用IPv4
	listener, err := net.Listen("tcp4", addr)
	if err != nil {
		log.Printf("WebSocket监听失败: %v", err)
		return
	}
	
	s.wsServer = &http.Server{
		Handler: mux,
	}
	
	log.Printf("WebSocket服务器启动在端口 %d (IPv4: %s)", s.config.Server.WSPort, addr)
	if err := s.wsServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Printf("WebSocket服务器错误: %v", err)
	}
}

// startHTTPServer 启动HTTP服务器
func (s *TunnelServer) startHTTPServer() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/clients", s.handleClients)
	mux.HandleFunc("/", s.handleHTTPRequest)
	
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.HTTPPort)
	
	// 创建TCP4监听器，强制使用IPv4
	listener, err := net.Listen("tcp4", addr)
	if err != nil {
		return fmt.Errorf("监听失败: %v", err)
	}
	
	s.httpServer = &http.Server{
		Handler: mux,
	}
	
	log.Printf("HTTP服务器启动在端口 %d (IPv4: %s)", s.config.Server.HTTPPort, addr)
	log.Printf("管理接口: http://localhost:%d/health", s.config.Server.HTTPPort)
	log.Printf("客户端列表: http://localhost:%d/clients", s.config.Server.HTTPPort)
	
	return s.httpServer.Serve(listener)
}

// handleWebSocket 处理WebSocket连接
func (s *TunnelServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 验证认证
	if s.config.Auth.RequireAuth {
		token := r.Header.Get("Authorization")
		if !s.validateToken(token) {
			http.Error(w, "认证失败", http.StatusUnauthorized)
			return
		}
	}
	
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}
	
	// 创建客户端
	clientID := fmt.Sprintf("client_%d", time.Now().Unix())
	host := r.Header.Get("X-Tunnel-Host")
	if host == "" {
		host = "localhost"
	}
	
	portStr := r.Header.Get("X-Tunnel-Port")
	port, _ := strconv.Atoi(portStr)
	if port == 0 {
		port = 3000
	}
	
	client := &Client{
		ID:       clientID,
		Conn:     conn,
		Host:     host,
		Port:     port,
		LastPing: time.Now(),
	}
	
	s.clientsMux.Lock()
	s.clients[clientID] = client
	s.clientsMux.Unlock()
	
	log.Printf("客户端连接: %s (%s:%d)", clientID, host, port)
	
	// 发送欢迎消息
	welcomeMsg := map[string]interface{}{
		"type": "connected",
		"data": map[string]interface{}{
			"clientId":     clientID,
			"publicUrl":    fmt.Sprintf("http://%s:%d", s.config.Server.PublicDomain, s.config.Server.HTTPPort),
			"localTarget": fmt.Sprintf("%s:%d", host, port),
		},
	}
	client.Conn.WriteJSON(welcomeMsg)
	
	// 处理消息
	defer func() {
		s.clientsMux.Lock()
		delete(s.clients, clientID)
		s.clientsMux.Unlock()
		conn.Close()
		log.Printf("客户端断开: %s", clientID)
	}()
	
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("读取消息失败: %v", err)
			break
		}
		
		// 处理不同类型的消息
		msgType, _ := msg["type"].(string)
		switch msgType {
		case "pong":
			client.LastPing = time.Now()
		case "http_response":
			s.handleHTTPResponse(msg)
		}
	}
}

// handleHTTPResponse 处理客户端的HTTP响应
func (s *TunnelServer) handleHTTPResponse(msg map[string]interface{}) {
	requestID, ok := msg["id"].(string)
	if !ok {
		log.Printf("HTTP响应缺少请求ID")
		return
	}
	
	data, ok := msg["data"].(map[string]interface{})
	if !ok {
		log.Printf("HTTP响应数据格式错误")
		return
	}
	
	// 构造响应对象
	response := HTTPResponse{}
	
	if statusCode, ok := data["statusCode"].(float64); ok {
		response.StatusCode = int(statusCode)
	}
	
	if headers, ok := data["headers"].(map[string]interface{}); ok {
		response.Headers = make(map[string]string)
		for k, v := range headers {
			if str, ok := v.(string); ok {
				response.Headers[k] = str
			}
		}
	}
	
	if body, ok := data["body"].(string); ok {
		response.Body = body
	}
	
	if errorMsg, ok := data["error"].(string); ok {
		response.Error = errorMsg
	}
	
	// 查找等待的请求通道
	s.requestMux.Lock()
	responseChan, exists := s.pendingRequests[requestID]
	if exists {
		delete(s.pendingRequests, requestID)
	}
	s.requestMux.Unlock()
	
	if !exists {
		log.Printf("未找到等待的请求: %s", requestID)
		return
	}
	
	// 发送响应到通道
	select {
	case responseChan <- response:
		log.Printf("HTTP响应已处理: %s (状态: %d)", requestID, response.StatusCode)
	default:
		log.Printf("响应通道已关闭: %s", requestID)
	}
}

// validateToken 验证令牌
func (s *TunnelServer) validateToken(authHeader string) bool {
	if authHeader == "" {
		return false
	}
	
	// 移除 "Bearer " 前缀
	token := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}
	
	for _, validToken := range s.config.Auth.Tokens {
		if token == validToken {
			return true
		}
	}
	return false
}

// handleHealth 健康检查
func (s *TunnelServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.clientsMux.RLock()
	clientCount := len(s.clients)
	s.clientsMux.RUnlock()
	
	response := map[string]interface{}{
		"status":  "healthy",
		"clients": clientCount,
		"uptime":  time.Now().Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleClients 客户端列表
func (s *TunnelServer) handleClients(w http.ResponseWriter, r *http.Request) {
	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()
	
	clients := make([]map[string]interface{}, 0)
	for _, client := range s.clients {
		clients = append(clients, map[string]interface{}{
			"id":        client.ID,
			"host":      client.Host,
			"port":      client.Port,
			"lastPing":  client.LastPing,
			"connected": true,
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

// handleHTTPRequest 处理HTTP请求转发
func (s *TunnelServer) handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	s.clientsMux.RLock()
	
	if len(s.clients) == 0 {
		s.clientsMux.RUnlock()
		http.Error(w, "没有可用的隧道客户端", http.StatusServiceUnavailable)
		return
	}
	
	// 选择第一个可用客户端 (简单负载均衡)
	var selectedClient *Client
	for _, client := range s.clients {
		selectedClient = client
		break
	}
	s.clientsMux.RUnlock()
	
	// 生成请求ID
	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	
	// 读取请求体
	bodyBytes := []byte{}
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "读取请求体失败", http.StatusBadRequest)
			return
		}
		r.Body.Close()
	}
	
	// 构造请求头映射
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	
	// 创建HTTP请求消息
	requestMsg := map[string]interface{}{
		"type": "http_request",
		"id":   requestID,
		"data": map[string]interface{}{
			"method":  r.Method,
			"url":     r.URL.Path,
			"query":   r.URL.RawQuery,
			"headers": headers,
			"body":    string(bodyBytes),
		},
	}
	
	// 创建响应通道
	responseChan := make(chan HTTPResponse, 1)
	s.requestMux.Lock()
	s.pendingRequests[requestID] = responseChan
	s.requestMux.Unlock()
	
	// 发送请求到客户端
	if err := selectedClient.Conn.WriteJSON(requestMsg); err != nil {
		s.requestMux.Lock()
		delete(s.pendingRequests, requestID)
		s.requestMux.Unlock()
		log.Printf("发送请求到客户端失败: %v", err)
		http.Error(w, "发送请求失败", http.StatusInternalServerError)
		return
	}
	
	log.Printf("转发请求到客户端: %s %s (ID: %s)", r.Method, r.URL.Path, requestID)
	
	// 等待响应
	select {
	case response := <-responseChan:
		// 清理等待的请求
		s.requestMux.Lock()
		delete(s.pendingRequests, requestID)
		s.requestMux.Unlock()
		
		// 处理错误响应
		if response.Error != "" {
			log.Printf("客户端响应错误: %s", response.Error)
			http.Error(w, response.Error, http.StatusBadGateway)
			return
		}
		
		// 设置响应头
		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}
		
		// 设置状态码
		w.WriteHeader(response.StatusCode)
		
		// 写入响应体
		if response.Body != "" {
			w.Write([]byte(response.Body))
		}
		
		log.Printf("响应已返回: %d (ID: %s)", response.StatusCode, requestID)
		
	case <-time.After(time.Duration(s.config.Server.RequestTimeout) * time.Millisecond):
		// 超时处理
		s.requestMux.Lock()
		delete(s.pendingRequests, requestID)
		s.requestMux.Unlock()
		
		log.Printf("请求超时: %s (ID: %s)", r.URL.Path, requestID)
		http.Error(w, "请求超时", http.StatusGatewayTimeout)
	}
}

var rootCmd = &cobra.Command{
	Use:   "tunnel-server",
	Short: "隧道服务器",
	Long:  "类似 Cloudflare Tunnel 的隧道服务器",
}

var serverCmd = &cobra.Command{
	Use:   "start",
	Short: "启动隧道服务器",
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		httpPort, _ := cmd.Flags().GetInt("http-port")
		wsPort, _ := cmd.Flags().GetInt("ws-port")
		host, _ := cmd.Flags().GetString("host")
		
		// 加载配置
		config, err := LoadConfig(configPath)
		if err != nil {
			log.Fatalf("加载配置失败: %v", err)
		}
		
		// 命令行参数覆盖配置文件
		if httpPort != 0 {
			config.Server.HTTPPort = httpPort
		}
		if wsPort != 0 {
			config.Server.WSPort = wsPort
		}
		if host != "" {
			config.Server.Host = host
		}
		
		fmt.Printf("启动隧道服务器...\n")
		fmt.Printf("HTTP 端口: %d\n", config.Server.HTTPPort)
		fmt.Printf("WebSocket 端口: %d\n", config.Server.WSPort)
		fmt.Printf("认证令牌数量: %d\n", len(config.Auth.Tokens))
		
		// 创建并启动服务器
		server := NewTunnelServer(config)
		if err := server.Start(); err != nil {
			log.Fatalf("启动服务器失败: %v", err)
		}
	},
}

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "令牌管理",
}

var addTokenCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "添加新令牌",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// 生成随机令牌
		token := fmt.Sprintf("token_%d_%s", time.Now().Unix(), args[0])
		fmt.Printf("✓ 新令牌已创建: %s - %s\n", args[0], token)
		fmt.Printf("请将此令牌添加到配置文件的 auth.tokens 数组中\n")
	},
}

var listTokenCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有令牌",
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		config, err := LoadConfig(configPath)
		if err != nil {
			log.Fatalf("加载配置失败: %v", err)
		}
		
		fmt.Printf("\n认证令牌列表:\n")
		fmt.Printf("─────────────────────────────────\n")
		for i, token := range config.Auth.Tokens {
			fmt.Printf("%d: %s\n", i+1, token)
		}
		fmt.Printf("\n总计: %d 个令牌\n", len(config.Auth.Tokens))
	},
}

func init() {
	// server 命令标志
	serverCmd.Flags().StringP("config", "c", "", "配置文件路径")
	serverCmd.Flags().Int("http-port", 0, "HTTP服务端口")
	serverCmd.Flags().Int("ws-port", 0, "WebSocket端口") 
	serverCmd.Flags().String("host", "", "监听地址")
	
	// token 命令标志
	tokenCmd.PersistentFlags().StringP("config", "c", "", "配置文件路径")
	
	// 添加子命令
	tokenCmd.AddCommand(addTokenCmd, listTokenCmd)
	rootCmd.AddCommand(serverCmd, tokenCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}