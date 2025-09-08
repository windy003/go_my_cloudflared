package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Config 客户端配置
type Config struct {
	Tunnel struct {
		URL              string `yaml:"url" json:"url"`
		AuthToken        string `yaml:"authToken" json:"authToken"`
		ReconnectAttempts int   `yaml:"reconnectAttempts" json:"reconnectAttempts"`
		ReconnectDelay    int   `yaml:"reconnectDelay" json:"reconnectDelay"`
		// TLS/SSL 配置
		InsecureSkipVerify bool   `yaml:"insecureSkipVerify" json:"insecureSkipVerify"`
		ServerName         string `yaml:"serverName" json:"serverName"`
		CACertFile         string `yaml:"caCertFile" json:"caCertFile"`
	} `yaml:"tunnel" json:"tunnel"`
	Local struct {
		Host string `yaml:"host" json:"host"`
		Port int    `yaml:"port" json:"port"`
	} `yaml:"local" json:"local"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	config := &Config{}
	config.Tunnel.URL = "ws://localhost:6001"
	config.Tunnel.AuthToken = "default-token"
	config.Tunnel.ReconnectAttempts = 10
	config.Tunnel.ReconnectDelay = 5000
	// TLS 默认配置
	config.Tunnel.InsecureSkipVerify = false
	config.Tunnel.ServerName = ""
	config.Tunnel.CACertFile = ""
	config.Local.Host = "localhost"
	config.Local.Port = 3000
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

// TunnelClient 隧道客户端
type TunnelClient struct {
	config          *Config
	conn            *websocket.Conn
	connected       bool
	reconnectCount  int
	stopChan        chan struct{}
	mu              sync.RWMutex
}

// NewTunnelClient 创建隧道客户端
func NewTunnelClient(config *Config) *TunnelClient {
	return &TunnelClient{
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// Start 启动客户端
func (c *TunnelClient) Start() error {
	log.Printf("启动隧道客户端...")
	log.Printf("连接地址: %s", c.config.Tunnel.URL)
	log.Printf("本地服务: %s:%d", c.config.Local.Host, c.config.Local.Port)
	
	// 启动连接
	if err := c.connect(); err != nil {
		return fmt.Errorf("初始连接失败: %v", err)
	}
	
	// 启动心跳
	go c.heartbeat()
	
	// 等待停止信号
	c.waitForStop()
	
	return nil
}

// connect 连接到隧道服务器
func (c *TunnelClient) connect() error {
	log.Printf("连接到隧道服务器: %s", c.config.Tunnel.URL)
	
	// 设置请求头
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+c.config.Tunnel.AuthToken)
	headers.Set("X-Tunnel-Host", c.config.Local.Host)
	headers.Set("X-Tunnel-Port", fmt.Sprintf("%d", c.config.Local.Port))
	
	// 创建WebSocket拨号器
	dialer := *websocket.DefaultDialer
	
	// 检查是否为WSS连接
	if strings.HasPrefix(c.config.Tunnel.URL, "wss://") {
		// 配置TLS
		tlsConfig := &tls.Config{
			InsecureSkipVerify: c.config.Tunnel.InsecureSkipVerify,
		}
		
		// 设置服务器名称
		if c.config.Tunnel.ServerName != "" {
			tlsConfig.ServerName = c.config.Tunnel.ServerName
		}
		
		// 如果指定了CA证书文件，这里可以加载（暂时跳过实现）
		if c.config.Tunnel.CACertFile != "" {
			log.Printf("注意: CA证书文件配置暂未实现: %s", c.config.Tunnel.CACertFile)
		}
		
		dialer.TLSClientConfig = tlsConfig
		log.Printf("使用WSS连接，InsecureSkipVerify: %v", c.config.Tunnel.InsecureSkipVerify)
	}
	
	// 建立WebSocket连接
	conn, _, err := dialer.Dial(c.config.Tunnel.URL, headers)
	if err != nil {
		return fmt.Errorf("WebSocket连接失败: %v", err)
	}
	
	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.reconnectCount = 0
	c.mu.Unlock()
	
	log.Printf("隧道连接已建立")
	
	// 启动消息处理
	go c.handleMessages()
	
	return nil
}

// handleMessages 处理消息
func (c *TunnelClient) handleMessages() {
	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.connected = false
		c.mu.Unlock()
		
		// 尝试重连
		go c.reconnect()
	}()
	
	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()
		
		if conn == nil {
			break
		}
		
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("读取消息失败: %v", err)
			break
		}
		
		// 处理不同类型的消息
		msgType, _ := msg["type"].(string)
		switch msgType {
		case "connected":
			c.handleConnected(msg)
		case "http_request":
			c.handleHTTPRequest(msg)
		case "ping":
			c.handlePing(msg)
		default:
			log.Printf("收到未知消息类型: %s", msgType)
		}
	}
}

// handleConnected 处理连接成功消息
func (c *TunnelClient) handleConnected(msg map[string]interface{}) {
	data, _ := msg["data"].(map[string]interface{})
	publicURL, _ := data["publicUrl"].(string)
	clientID, _ := data["clientId"].(string)
	
	log.Printf("✓ 隧道已建立")
	log.Printf("  客户端ID: %s", clientID)
	if publicURL != "" {
		log.Printf("  公网地址: %s", publicURL)
	}
	log.Printf("  本地服务: %s:%d", c.config.Local.Host, c.config.Local.Port)
}

// handleHTTPRequest 处理HTTP请求
func (c *TunnelClient) handleHTTPRequest(msg map[string]interface{}) {
	data, _ := msg["data"].(map[string]interface{})
	requestID, _ := msg["id"].(string)
	method, _ := data["method"].(string)
	url, _ := data["url"].(string)
	query, _ := data["query"].(string)
	headers, _ := data["headers"].(map[string]interface{})
	body, _ := data["body"].(string)
	
	log.Printf("处理请求: %s %s", method, url)
	
	// 构建完整的本地URL
	localURL := fmt.Sprintf("http://%s:%d%s", c.config.Local.Host, c.config.Local.Port, url)
	if query != "" {
		localURL += "?" + query
	}
	
	// 创建HTTP请求
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	
	req, err := http.NewRequest(method, localURL, reqBody)
	if err != nil {
		c.sendErrorResponse(requestID, fmt.Sprintf("创建请求失败: %v", err))
		return
	}
	
	// 设置请求头
	if headers != nil {
		for k, v := range headers {
			if str, ok := v.(string); ok {
				req.Header.Set(k, str)
			}
		}
	}
	
	// 执行HTTP请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("请求本地服务失败: %v", err)
		c.sendErrorResponse(requestID, fmt.Sprintf("请求本地服务失败: %v", err))
		return
	}
	defer resp.Body.Close()
	
	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取响应体失败: %v", err)
		c.sendErrorResponse(requestID, fmt.Sprintf("读取响应体失败: %v", err))
		return
	}
	
	// 构造响应头映射
	responseHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			responseHeaders[k] = v[0]
		}
	}
	
	// 发送成功响应
	response := map[string]interface{}{
		"type": "http_response",
		"id":   requestID,
		"data": map[string]interface{}{
			"statusCode": resp.StatusCode,
			"headers":    responseHeaders,
			"body":       string(respBody),
		},
	}
	
	c.mu.RLock()
	if c.conn != nil {
		err = c.conn.WriteJSON(response)
		if err != nil {
			log.Printf("发送响应失败: %v", err)
		} else {
			log.Printf("响应已发送: %d %s (ID: %s)", resp.StatusCode, localURL, requestID)
		}
	}
	c.mu.RUnlock()
}

// sendErrorResponse 发送错误响应
func (c *TunnelClient) sendErrorResponse(requestID, errorMsg string) {
	response := map[string]interface{}{
		"type": "http_response",
		"id":   requestID,
		"data": map[string]interface{}{
			"statusCode": 500,
			"headers": map[string]string{
				"Content-Type": "text/plain",
			},
			"body":  "",
			"error": errorMsg,
		},
	}
	
	c.mu.RLock()
	if c.conn != nil {
		c.conn.WriteJSON(response)
		log.Printf("错误响应已发送: %s (ID: %s)", errorMsg, requestID)
	}
	c.mu.RUnlock()
}

// handlePing 处理心跳
func (c *TunnelClient) handlePing(msg map[string]interface{}) {
	pingID, _ := msg["id"].(string)
	
	pong := map[string]interface{}{
		"type": "pong",
		"id":   pingID,
	}
	
	c.mu.RLock()
	if c.conn != nil {
		c.conn.WriteJSON(pong)
	}
	c.mu.RUnlock()
}

// heartbeat 心跳检测
func (c *TunnelClient) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			c.mu.RLock()
			connected := c.connected
			c.mu.RUnlock()
			
			if !connected {
				continue
			}
			
			// 发送心跳
			heartbeat := map[string]interface{}{
				"type": "heartbeat",
				"time": time.Now().Unix(),
			}
			
			c.mu.RLock()
			if c.conn != nil {
				c.conn.WriteJSON(heartbeat)
			}
			c.mu.RUnlock()
			
		case <-c.stopChan:
			return
		}
	}
}

// reconnect 重连
func (c *TunnelClient) reconnect() {
	c.mu.Lock()
	c.reconnectCount++
	count := c.reconnectCount
	c.mu.Unlock()
	
	if count > c.config.Tunnel.ReconnectAttempts {
		log.Printf("达到最大重连次数 (%d)，停止重连", c.config.Tunnel.ReconnectAttempts)
		return
	}
	
	delay := time.Duration(c.config.Tunnel.ReconnectDelay*count) * time.Millisecond
	log.Printf("尝试重连 (%d/%d)，等待 %v...", count, c.config.Tunnel.ReconnectAttempts, delay)
	
	time.Sleep(delay)
	
	if err := c.connect(); err != nil {
		log.Printf("重连失败: %v", err)
		go c.reconnect() // 继续尝试
	}
}

// Stop 停止客户端
func (c *TunnelClient) Stop() {
	log.Printf("停止隧道客户端...")
	
	close(c.stopChan)
	
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connected = false
	c.mu.Unlock()
	
	log.Printf("隧道客户端已停止")
}

// waitForStop 等待停止信号
func (c *TunnelClient) waitForStop() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	select {
	case sig := <-sigChan:
		log.Printf("收到信号 %v，正在停止...", sig)
		c.Stop()
	case <-c.stopChan:
		// 正常停止
	}
}

var rootCmd = &cobra.Command{
	Use:   "tunnel-client",
	Short: "隧道客户端",
	Long:  "类似 Cloudflare Tunnel 的隧道客户端",
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "运行隧道客户端",
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		tunnelURL, _ := cmd.Flags().GetString("tunnel-url")
		authToken, _ := cmd.Flags().GetString("auth-token")
		localHost, _ := cmd.Flags().GetString("local-host")
		localPort, _ := cmd.Flags().GetInt("local-port")
		
		// 加载配置
		config, err := LoadConfig(configPath)
		if err != nil {
			log.Fatalf("加载配置失败: %v", err)
		}
		
		// 命令行参数覆盖配置文件
		if tunnelURL != "" {
			config.Tunnel.URL = tunnelURL
		}
		if authToken != "" {
			config.Tunnel.AuthToken = authToken
		}
		if localHost != "" {
			config.Local.Host = localHost
		}
		if localPort != 0 {
			config.Local.Port = localPort
		}
		
		// 创建并启动客户端
		client := NewTunnelClient(config)
		if err := client.Start(); err != nil {
			log.Fatalf("启动客户端失败: %v", err)
		}
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
}

var initConfigCmd = &cobra.Command{
	Use:   "init",
	Short: "创建示例配置文件",
	Run: func(cmd *cobra.Command, args []string) {
		config := map[string]interface{}{
			"tunnel": map[string]interface{}{
				"url":               "wss://windy.run:6444",
				"authToken":         "your-auth-token-here",
				"reconnectAttempts": 10,
				"reconnectDelay":    5000,
				// WSS/TLS 配置
				"insecureSkipVerify": true,  // 自签名证书时设为true
				"serverName":         "",    // 可选：指定服务器名称
				"caCertFile":         "",    // 可选：CA证书文件路径
			},
			"local": map[string]interface{}{
				"host": "localhost",
				"port": 3000,
			},
		}
		
		data, _ := json.MarshalIndent(config, "", "  ")
		filename := "tunnel.json"
		
		if err := os.WriteFile(filename, data, 0644); err != nil {
			log.Fatalf("创建配置文件失败: %v", err)
		}
		
		fmt.Printf("✓ 示例配置文件已创建: %s\n", filename)
		fmt.Printf("请编辑此文件以配置你的隧道连接\n")
	},
}

var showConfigCmd = &cobra.Command{
	Use:   "show",
	Short: "显示当前配置",
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		config, err := LoadConfig(configPath)
		if err != nil {
			log.Fatalf("加载配置失败: %v", err)
		}
		
		data, _ := json.MarshalIndent(config, "", "  ")
		fmt.Printf("当前配置:\n%s\n", string(data))
	},
}

func init() {
	// run 命令标志
	runCmd.Flags().StringP("config", "c", "", "配置文件路径")
	runCmd.Flags().String("tunnel-url", "", "隧道服务器URL")
	runCmd.Flags().String("auth-token", "", "认证令牌")
	runCmd.Flags().String("local-host", "", "本地服务主机")
	runCmd.Flags().Int("local-port", 0, "本地服务端口")
	
	// config 命令标志
	configCmd.PersistentFlags().StringP("config", "c", "", "配置文件路径")
	
	// 添加子命令
	configCmd.AddCommand(initConfigCmd, showConfigCmd)
	rootCmd.AddCommand(runCmd, configCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}