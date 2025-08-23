// Package adapters 实现了各种代理协议的适配器
package adapters

import (
	"io"
	"net"

	C "github.com/fossabot/clash/constant"
)

// DirectAdapter 是一个直接连接的适配器
// 它实现了 ProxyAdapter 接口，用于处理不通过代理直接连接目标的网络请求
type DirectAdapter struct {
	conn net.Conn // 底层的网络连接
}

// ReadWriter 返回用于处理网络流量的读写器
// 这个方法实现了 ProxyAdapter 接口，允许上层代码通过统一接口读写数据
func (d *DirectAdapter) ReadWriter() io.ReadWriter {
	return d.conn
}

// Close 用于关闭直接连接
// 这个方法实现了 ProxyAdapter 接口，确保连接可以被正确关闭
func (d *DirectAdapter) Close() {
	d.conn.Close()
}

// Conn 返回底层的网络连接
// 这个方法实现了 ProxyAdapter 接口，提供对原始连接的访问
func (d *DirectAdapter) Conn() net.Conn {
	return d.conn
}

// Direct 结构体表示一个直接连接代理
// 它实现了 Proxy 接口，用于创建直接连接的适配器实例
type Direct struct {
	traffic *C.Traffic // 流量统计器，用于跟踪上传和下载的字节数
}

// Name 返回代理的名称
// 这个方法实现了 Proxy 接口
func (d *Direct) Name() string {
	return "Direct"
}

// Generator 根据目标地址生成一个直接连接的适配器
// 这个方法实现了 Proxy 接口，用于创建到目标地址的直接连接
// addr: 目标地址信息
// 返回: 直接连接适配器和可能的错误
func (d *Direct) Generator(addr *C.Addr) (adapter C.ProxyAdapter, err error) {
	// 建立到目标地址的 TCP 连接
	// 使用 net.JoinHostPort 将主机名和端口组合成 "host:port" 格式
	c, err := net.Dial("tcp", net.JoinHostPort(addr.String(), addr.Port))
	if err != nil {
		return
	}

	// 设置 TCP 连接的 KeepAlive 属性，保持连接活跃
	c.(*net.TCPConn).SetKeepAlive(true)

	// 创建带流量统计的连接跟踪器，并返回直接连接适配器
	// NewTrafficTrack 包装原始连接以跟踪流量使用情况
	return &DirectAdapter{conn: NewTrafficTrack(c, d.traffic)}, nil
}

// NewDirect 创建一个新的直接连接代理实例
// traffic: 流量统计器
// 返回: 配置好的 Direct 代理实例
func NewDirect(traffic *C.Traffic) *Direct {
	return &Direct{traffic: traffic}
}
