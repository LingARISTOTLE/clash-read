// Package adapters 实现了各种代理协议的适配器
package adapters

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"

	C "../constant"

	"github.com/riobard/go-shadowsocks2/core"
	"github.com/riobard/go-shadowsocks2/socks"
)

// ShadowsocksAdapter 是一个 Shadowsocks 适配器，用于处理已建立的连接
type ShadowsocksAdapter struct {
	conn net.Conn
}

// ReadWriter 返回用于处理网络流量的读写器
// 这个方法实现了 ProxyAdapter 接口，允许上层代码通过统一接口读写数据
func (ss *ShadowsocksAdapter) ReadWriter() io.ReadWriter {
	return ss.conn
}

// Close 关闭 Shadowsocks 连接
// 这个方法实现了 ProxyAdapter 接口，确保连接可以被正确关闭
func (ss *ShadowsocksAdapter) Close() {
	ss.conn.Close()
}

// Conn 返回底层的网络连接
func (ss *ShadowsocksAdapter) Conn() net.Conn {
	return ss.conn
}

// ShadowSocks 结构体表示一个 Shadowsocks 代理配置
type ShadowSocks struct {
	server  string      // Shadowsocks 服务器地址 (host:port)
	name    string      // 代理名称
	cipher  core.Cipher // 加密器，用于加密和解密数据
	traffic *C.Traffic  // 流量统计器，用于跟踪上传和下载的字节数
}

// Name 返回 Shadowsocks 代理的名称
func (ss *ShadowSocks) Name() string {
	return ss.name
}

// Generator 根据目标地址生成一个 Shadowsocks 连接适配器
// 这个方法实现了 Proxy 接口，用于创建到目标地址的连接
func (ss *ShadowSocks) Generator(addr *C.Addr) (adapter C.ProxyAdapter, err error) {
	// 建立到 Shadowsocks 服务器的 TCP 连接
	c, err := net.Dial("tcp", ss.server)
	if err != nil {
		return nil, fmt.Errorf("%s connect error", ss.server)
	}

	// 设置 TCP 连接的 KeepAlive 属性，保持连接活跃
	c.(*net.TCPConn).SetKeepAlive(true)

	// 使用配置的加密器包装原始连接，实现 Shadowsocks 加密通信
	c = ss.cipher.StreamConn(c)

	// 将目标地址信息写入连接，告诉 Shadowsocks 服务器要连接哪个目标
	_, err = c.Write(serializesSocksAddr(addr))

	// 创建带流量统计的连接跟踪器，并返回 Shadowsocks 适配器
	return &ShadowsocksAdapter{conn: NewTrafficTrack(c, ss.traffic)}, err
}

// NewShadowSocks 创建一个新的 Shadowsocks 代理实例
// name: 代理名称
// ssURL: Shadowsocks URL 格式，如 "ss://method:password@server:port"
// traffic: 流量统计器
func NewShadowSocks(name string, ssURL string, traffic *C.Traffic) (*ShadowSocks, error) {
	var key []byte

	// 解析 Shadowsocks URL，提取服务器地址、加密方法和密码
	server, cipher, password, _ := parseURL(ssURL)

	// 根据加密方法、密钥和密码创建加密器
	ciph, err := core.PickCipher(cipher, key, password)
	if err != nil {
		return nil, fmt.Errorf("ss %s initialize error: %s", server, err.Error())
	}

	// 返回配置好的 Shadowsocks 代理实例
	return &ShadowSocks{
		server:  server,
		name:    name,
		cipher:  ciph,
		traffic: traffic,
	}, nil
}

// parseURL 解析 Shadowsocks URL 格式
// 输入格式: ss://method:password@server:port
// 返回: 服务器地址、加密方法、密码和可能的错误
func parseURL(s string) (addr, cipher, password string, err error) {
	// 使用标准库解析 URL
	u, err := url.Parse(s)
	if err != nil {
		return
	}

	// 提取服务器地址 (host:port)
	addr = u.Host

	// 如果 URL 中包含用户信息，则提取加密方法和密码
	if u.User != nil {
		// 用户名部分是加密方法
		cipher = u.User.Username()
		// 密码部分
		password, _ = u.User.Password()
	}
	return
}

// serializesSocksAddr 将地址信息序列化为 SOCKS 地址格式
// 这是 Shadowsocks 协议要求的格式，用于告诉服务器要连接的目标地址
func serializesSocksAddr(addr *C.Addr) []byte {
	var buf [][]byte

	// 获取地址类型 (域名、IPv4 或 IPv6)
	aType := uint8(addr.AddrType)

	// 将端口转换为网络字节序 (大端序)
	p, _ := strconv.Atoi(addr.Port)
	port := []byte{uint8(p >> 8), uint8(p & 0xff)}

	// 根据地址类型处理不同的地址格式
	switch addr.AddrType {
	case socks.AtypDomainName:
		// 域名地址: 类型 + 长度 + 域名 + 端口
		len := uint8(len(addr.Host))
		host := []byte(addr.Host)
		buf = [][]byte{{aType, len}, host, port}
	case socks.AtypIPv4:
		// IPv4 地址: 类型 + 4字节IP + 端口
		host := addr.IP.To4()
		buf = [][]byte{{aType}, host, port}
	case socks.AtypIPv6:
		// IPv6 地址: 类型 + 16字节IP + 端口
		host := addr.IP.To16()
		buf = [][]byte{{aType}, host, port}
	}

	// 将所有部分连接成一个完整的地址字节流
	return bytes.Join(buf, []byte(""))
}
