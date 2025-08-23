# Clash启动流程分析

## 功能概述
Clash是一个基于规则的网络代理工具，支持HTTP/HTTPS和SOCKS代理服务。本文档详细分析了Clash的启动流程，包括配置加载、代理服务初始化和核心组件启动等过程。

## 入口方法
`main.main`

## 方法调用树
```
main.main
 ├── C.GetConfig
 ├── tunnel.GetInstance().UpdateConfig
 ├── http.NewHttpProxy
 ├── socks.NewSocksProxy
 ├── hub.NewHub (可选)
 └── signal.Notify
```

## 详细业务流程

1. **配置加载**
   - 调用`C.GetConfig()`方法读取配置文件
   - 配置文件默认路径为`$HOME/.config/clash/config.ini`
   - 如果配置文件不存在，则创建一个空的配置文件

2. **端口配置解析**
   - 从配置文件的`[General]`部分读取HTTP和SOCKS代理端口
   - 默认HTTP端口为7890，默认SOCKS端口为7891
   - 如果配置文件中没有指定端口，则使用默认端口

3. **隧道配置更新**
   - 获取Tunnel单例实例并调用`UpdateConfig()`方法
   - 解析配置文件中的`[Proxy]`、`[Proxy Group]`和`[Rule]`部分
   - 初始化所有代理服务器和规则

4. **代理服务启动**
   - 启动HTTP代理服务监听指定端口
   - 启动SOCKS代理服务监听指定端口
   - 如果配置了外部控制器，则启动控制器服务

5. **信号监听**
   - 注册SIGINT和SIGTERM信号处理
   - 等待退出信号以优雅关闭程序

## 关键业务规则

- **配置优先级规则**：配置文件中的设置优先于默认值
- **代理服务规则**：HTTP和SOCKS代理服务必须在不同端口上运行
- **信号处理规则**：程序接收到SIGINT或SIGTERM信号时应该优雅退出
- **单例模式规则**：Tunnel实例在整个程序中应该是唯一的

## 数据流转

- **输入**：配置文件(config.ini)、命令行信号
- **处理**：解析配置、初始化代理和规则、启动网络服务
- **输出**：运行中的代理服务、日志输出、流量统计

## 扩展点/分支逻辑

- **配置文件分支**：如果配置文件不存在则创建默认配置文件
- **外部控制器分支**：如果配置了external-controller则启动控制器服务
- **代理类型分支**：支持多种代理类型（Direct、Reject、Shadowsocks等）
- **规则匹配分支**：支持多种规则类型（DomainSuffix、DomainKeyword、GEOIP、IPCIDR、FINAL）

## 外部依赖

- **配置管理**：使用`gopkg.in/ini.v1`包处理INI格式配置文件
- **日志系统**：使用`github.com/sirupsen/logrus`包处理日志
- **网络库**：使用Go标准库`net`包处理网络连接
- **Shadowsocks支持**：使用`github.com/riobard/go-shadowsocks2`包实现Shadowsocks协议
- **GeoIP支持**：使用`github.com/oschwald/geoip2-golang`包实现地理位置查询

## 注意事项

- 程序需要能够访问配置目录以读取和写入配置文件
- GeoIP数据库文件需要正确下载并放置在配置目录中
- 启动时需要确保指定的端口未被其他程序占用
- 程序运行时需要网络访问权限以连接到代理服务器