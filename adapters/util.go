// Package adapters 实现了各种代理协议的适配器
package adapters

import (
	"net"

	C "github.com/fossabot/clash/constant"
)

// TrafficTrack 是一个流量统计跟踪器
// 它通过嵌入 net.Conn 接口来包装原始连接，并在数据读写时统计流量
type TrafficTrack struct {
	net.Conn            // 嵌入原始连接接口，获得所有网络连接方法
	traffic  *C.Traffic // 流量统计器，用于记录上传和下载流量
}

// Read 从连接中读取数据并统计下载流量
// b: 用于存储读取数据的字节切片
// 返回: 实际读取的字节数和可能的错误
func (tt *TrafficTrack) Read(b []byte) (int, error) {
	// 从原始连接读取数据
	n, err := tt.Conn.Read(b)

	// 将读取的字节数发送到流量统计器的下载通道
	// 这会增加下载流量计数
	tt.traffic.Down() <- int64(n)

	// 返回实际读取的字节数和可能的错误
	return n, err
}

// Write 向连接中写入数据并统计上传流量
// b: 要写入的数据
// 返回: 实际写入的字节数和可能的错误
func (tt *TrafficTrack) Write(b []byte) (int, error) {
	// 向原始连接写入数据
	n, err := tt.Conn.Write(b)

	// 将写入的字节数发送到流量统计器的上传通道
	// 这会增加上传流量计数
	tt.traffic.Up() <- int64(n)

	// 返回实际写入的字节数和可能的错误
	return n, err
}

// NewTrafficTrack 创建一个新的流量统计跟踪器
// conn: 原始网络连接
// traffic: 流量统计器
// 返回: 包装了流量统计功能的连接
func NewTrafficTrack(conn net.Conn, traffic *C.Traffic) *TrafficTrack {
	// 返回一个新的 TrafficTrack 实例
	// 通过嵌入 net.Conn 和组合 Traffic 实例来实现流量统计功能
	return &TrafficTrack{traffic: traffic, Conn: conn}
}
