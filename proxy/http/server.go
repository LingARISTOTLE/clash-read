// Package http 实现了HTTP代理服务端功能
// 包括HTTP服务器启动、请求处理和HTTPS隧道建立等功能
package http

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	C "../../constant"
	"../../tunnel"

	"github.com/riobard/go-shadowsocks2/socks"
	log "github.com/sirupsen/logrus"
)

// 全局变量，获取Tunnel单例实例
// 用于将HTTP请求转发到Tunnel模块进行处理
var (
	tun = tunnel.GetInstance()
)

// NewHttpProxy 创建并启动HTTP代理服务器
// port: 监听端口号
func NewHttpProxy(port string) {
	// 创建HTTP服务器实例
	server := &http.Server{
		// 设置服务器监听地址
		Addr: fmt.Sprintf(":%s", port),
		// 设置请求处理器
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 根据请求方法进行不同处理
			if r.Method == http.MethodConnect {
				// HTTPS CONNECT请求处理
				handleTunneling(w, r)
			} else {
				// 普通HTTP请求处理
				handleHTTP(w, r)
			}
		}),
	}

	// 记录日志信息
	log.Infof("HTTP proxy :%s", port)

	// 启动HTTP服务器并开始监听
	server.ListenAndServe()
}

// handleHTTP 处理普通的HTTP请求
// w: HTTP响应写入器
// r: HTTP请求
func handleHTTP(w http.ResponseWriter, r *http.Request) {
	// 获取目标地址
	addr := r.Host

	// 如果地址中没有端口号，则添加默认的HTTP端口80
	// padding default port
	if !strings.Contains(addr, ":") {
		addr += ":80"
	}

	// 创建HTTP适配器实例
	req, done := NewHttp(addr, w, r)

	// 将请求添加到Tunnel处理队列中
	tun.Add(req)

	// 等待请求处理完成
	<-done
}

// handleTunneling 处理HTTPS的CONNECT请求，建立隧道连接
// w: HTTP响应写入器
// r: HTTP请求
func handleTunneling(w http.ResponseWriter, r *http.Request) {
	// 检查响应写入器是否实现了Hijacker接口
	// Hijacker接口允许接管底层连接
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		// 如果不支持连接劫持，则直接返回
		return
	}

	// 劫持HTTP连接，获取底层TCP连接
	conn, _, err := hijacker.Hijack()
	if err != nil {
		// 如果劫持连接失败，则直接返回
		return
	}

	// 向客户端发送连接建立成功的响应
	// 注意：不能使用w.WriteHeader(http.StatusOK)，因为在Safari浏览器中不工作
	conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

	// 创建HTTPS适配器并添加到Tunnel处理队列
	tun.Add(NewHttps(r.Host, conn))
}

// parseHttpAddr 解析HTTP目标地址并创建Addr结构体
// target: 目标地址字符串，格式为"host:port"
// 返回: 解析后的地址结构体
func parseHttpAddr(target string) *C.Addr {
	// 分割主机名和端口号
	host, port, _ := net.SplitHostPort(target)

	// 解析IP地址
	ipAddr, err := net.ResolveIPAddr("ip", host)
	var resolveIP *net.IP
	if err == nil {
		// 如果解析成功，保存解析后的IP地址
		resolveIP = &ipAddr.IP
	}

	// 确定地址类型
	var addType int
	ip := net.ParseIP(host)
	switch {
	case ip == nil:
		// 如果不是IP地址，则为域名类型
		addType = socks.AtypDomainName
	case ip.To4() == nil:
		// 如果不是IPv4地址，则为IPv6地址类型
		addType = socks.AtypIPv6
	default:
		// 默认为IPv4地址类型
		addType = socks.AtypIPv4
	}

	// 创建并返回地址结构体
	return &C.Addr{
		NetWork:  C.TCP,     // 网络类型为TCP
		AddrType: addType,   // 地址类型（域名、IPv4或IPv6）
		Host:     host,      // 主机名
		IP:       resolveIP, // 解析后的IP地址
		Port:     port,      // 端口号
	}
}
