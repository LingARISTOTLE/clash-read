// Package http 实现了HTTP代理功能
// 处理HTTP和HTTPS CONNECT请求，将客户端请求转发到目标服务器
package http

import (
	"io"
	"net"
	"net/http"
	"time"

	C "../../constant"
)

// HttpAdapter 是HTTP代理适配器
// 实现了ServerAdapter接口，用于处理HTTP连接请求
type HttpAdapter struct {
	addr *C.Addr             // 目标地址信息
	r    *http.Request       // 原始HTTP请求
	w    http.ResponseWriter // HTTP响应写入器
	done chan struct{}       // 完成通知通道
}

// Close 关闭HTTP适配器连接
// 通过向done通道发送信号通知连接处理完成
// 这个方法实现了ServerAdapter接口
func (h *HttpAdapter) Close() {
	h.done <- struct{}{}
}

// Addr 返回目标地址信息
// 这个方法实现了ServerAdapter接口
func (h *HttpAdapter) Addr() *C.Addr {
	return h.addr
}

// Connect 建立连接并转发HTTP请求到目标服务器
// proxy: 代理适配器，用于建立到目标服务器的连接
// 这个方法实现了ServerAdapter接口
func (h *HttpAdapter) Connect(proxy C.ProxyAdapter) {
	// 创建HTTP传输对象，用于转发请求到目标服务器
	req := http.Transport{
		// 自定义Dial函数，使用代理适配器建立连接
		Dial: func(string, string) (net.Conn, error) {
			return proxy.Conn(), nil
		},
		// 以下参数来自http.DefaultTransport的默认配置
		MaxIdleConns:          100,              // 最大空闲连接数
		IdleConnTimeout:       90 * time.Second, // 空闲连接超时时间
		ExpectContinueTimeout: 1 * time.Second,  // Expect请求超时时间
	}

	// 使用传输对象发送HTTP请求到目标服务器
	resp, err := req.RoundTrip(h.r)
	if err != nil {
		// 如果请求失败，直接返回
		return
	}
	// 函数结束时关闭响应体
	defer resp.Body.Close()

	// 将目标服务器的响应头复制到客户端响应中
	header := h.w.Header()
	for k, vv := range resp.Header {
		for _, v := range vv {
			header.Add(k, v)
		}
	}

	// 设置响应状态码并写入响应头
	h.w.WriteHeader(resp.StatusCode)

	// 根据传输编码类型选择合适的写入器
	var writer io.Writer = h.w
	if len(resp.TransferEncoding) > 0 && resp.TransferEncoding[0] == "chunked" {
		// 如果是分块传输编码，使用ChunkWriter处理
		writer = ChunkWriter{Writer: h.w}
	}

	// 将目标服务器的响应体数据复制到客户端
	io.Copy(writer, resp.Body)
}

// ChunkWriter 是分块传输编码的写入器
// 用于处理HTTP分块传输编码的响应数据
type ChunkWriter struct {
	io.Writer // 嵌入Writer接口，获得Write方法
}

// Write 实现io.Writer接口的Write方法
// 写入数据并在成功后刷新缓冲区
func (cw ChunkWriter) Write(b []byte) (int, error) {
	// 写入数据
	n, err := cw.Writer.Write(b)
	if err == nil {
		// 如果写入成功，刷新缓冲区确保数据立即发送
		cw.Writer.(http.Flusher).Flush()
	}
	return n, err
}

// NewHttp 创建一个新的HTTP适配器实例
// host: 目标主机地址
// w: HTTP响应写入器
// r: 原始HTTP请求
// 返回: HTTP适配器实例和完成通知通道
func NewHttp(host string, w http.ResponseWriter, r *http.Request) (*HttpAdapter, chan struct{}) {
	// 创建完成通知通道
	done := make(chan struct{})

	// 返回配置好的HTTP适配器实例
	return &HttpAdapter{
		addr: parseHttpAddr(host), // 解析目标地址
		r:    r,                   // 保存原始请求
		w:    w,                   // 保存响应写入器
		done: done,                // 保存完成通道
	}, done
}
