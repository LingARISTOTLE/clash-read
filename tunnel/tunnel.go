package tunnel

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fossabot/clash/adapters"
	C "github.com/fossabot/clash/constant"
	"github.com/fossabot/clash/observable"
	R "github.com/fossabot/clash/rules"

	"gopkg.in/eapache/channels.v1"
)

// 全局变量，用于确保tunnel实例只被创建一次
var (
	tunnel *Tunnel
	once   sync.Once
)

// Tunnel 结构体是整个代理隧道的核心
// 它负责处理连接请求、规则匹配和代理选择
type Tunnel struct {
	// queue 是一个无限缓冲通道，用于存放待处理的连接请求
	queue *channels.InfiniteChannel

	// rules 存储所有配置的规则，按优先级排序
	rules []C.Rule

	// proxys 存储所有可用的代理，包括DIRECT、REJECT和各种代理服务器
	proxys map[string]C.Proxy

	// observable 用于日志观察，允许外部订阅日志事件
	observable *observable.Observable

	// logCh 是日志通道，用于传递日志消息
	logCh chan interface{}

	// configLock 读写锁，保护配置的并发访问
	configLock *sync.RWMutex

	// traffic 用于统计流量信息
	traffic *C.Traffic
}

// Add 方法将一个新的连接请求添加到处理队列中
// 参数 req 是一个服务器适配器接口，代表一个待处理的连接
func (t *Tunnel) Add(req C.ServerAdapter) {
	t.queue.In() <- req
}

// Traffic 方法返回当前的流量统计信息
func (t *Tunnel) Traffic() *C.Traffic {
	return t.traffic
}

// Config 方法返回当前的规则和代理配置
// 返回规则列表和代理映射
func (t *Tunnel) Config() ([]C.Rule, map[string]C.Proxy) {
	return t.rules, t.proxys
}

// Log 方法返回日志观察对象，允许外部订阅日志
func (t *Tunnel) Log() *observable.Observable {
	return t.observable
}

// UpdateConfig 方法从配置文件更新隧道配置
// 包括代理、规则和代理组的配置
func (t *Tunnel) UpdateConfig() (err error) {
	// 获取当前配置
	cfg, err := C.GetConfig()
	if err != nil {
		return
	}

	// 初始化空的代理和规则映射
	proxys := make(map[string]C.Proxy)
	rules := []C.Rule{}

	// 获取配置中的各部分
	proxysConfig := cfg.Section("Proxy")
	rulesConfig := cfg.Section("Rule")
	groupsConfig := cfg.Section("Proxy Group")

	// 解析代理配置
	for _, key := range proxysConfig.Keys() {
		// 将代理配置按逗号分割
		proxy := strings.Split(key.Value(), ",")
		if len(proxy) == 0 {
			continue
		}
		proxy = trimArr(proxy)
		// 根据代理类型进行处理
		switch proxy[0] {
		// 处理Shadowsocks代理 ss, server, port, cipher, password
		case "ss":
			if len(proxy) < 5 {
				continue
			}
			// 构造Shadowsocks URL
			ssURL := fmt.Sprintf("ss://%s:%s@%s:%s", proxy[3], proxy[4], proxy[1], proxy[2])
			// 创建Shadowsocks代理适配器
			ss, err := adapters.NewShadowSocks(key.Name(), ssURL, t.traffic)
			if err != nil {
				return err
			}
			proxys[key.Name()] = ss
		}
	}

	// 解析规则配置
	for _, key := range rulesConfig.Keys() {
		// 将规则按逗号分割
		rule := strings.Split(key.Name(), ",")
		if len(rule) < 3 {
			continue
		}
		rule = trimArr(rule)
		// 根据规则类型进行处理
		switch rule[0] {
		case "DOMAIN-SUFFIX":
			// 域名后缀匹配规则
			rules = append(rules, R.NewDomainSuffix(rule[1], rule[2]))
		case "DOMAIN-KEYWORD":
			// 域名关键字匹配规则
			rules = append(rules, R.NewDomainKeyword(rule[1], rule[2]))
		case "GEOIP":
			// 地理位置IP匹配规则
			rules = append(rules, R.NewGEOIP(rule[1], rule[2]))
		case "IP-CIDR", "IP-CIDR6":
			// IP地址段匹配规则
			rules = append(rules, R.NewIPCIDR(rule[1], rule[2]))
		case "FINAL":
			// 最终匹配规则（默认规则）
			rules = append(rules, R.NewFinal(rule[2]))
		}
	}

	// 解析代理组配置
	for _, key := range groupsConfig.Keys() {
		// 将代理组配置按逗号分割
		rule := strings.Split(key.Value(), ",")
		if len(rule) < 4 {
			continue
		}
		rule = trimArr(rule)
		// 根据代理组类型进行处理
		switch rule[0] {
		case "url-test":
			// URL测试代理组
			proxyNames := rule[1 : len(rule)-2]
			delay, _ := strconv.Atoi(rule[len(rule)-1])
			url := rule[len(rule)-2]
			var ps []C.Proxy
			// 收集代理组中包含的代理
			for _, name := range proxyNames {
				if p, ok := proxys[name]; ok {
					ps = append(ps, p)
				}
			}

			// 创建URL测试适配器
			adapter, err := adapters.NewURLTest(key.Name(), ps, url, time.Duration(delay)*time.Second)
			if err != nil {
				return fmt.Errorf("Config error: %s", err.Error())
			}
			proxys[key.Name()] = adapter
		}
	}

	// 初始化内置代理
	proxys["DIRECT"] = adapters.NewDirect(t.traffic) // 直连代理
	proxys["REJECT"] = adapters.NewReject()          // 拒绝代理

	// 加写锁保护配置更新
	t.configLock.Lock()
	defer t.configLock.Unlock()

	// 停止旧的url-test代理
	for _, elm := range t.proxys {
		urlTest, ok := elm.(*adapters.URLTest)
		if ok {
			urlTest.Close()
		}
	}

	// 更新代理和规则配置
	t.proxys = proxys
	t.rules = rules

	return nil
}

// process 方法是隧道的核心处理循环
// 它从队列中取出连接请求并异步处理
func (t *Tunnel) process() {
	queue := t.queue.Out()
	for {
		// 从队列中取出一个连接请求
		elm := <-queue
		conn := elm.(C.ServerAdapter)
		// 异步处理连接
		go t.handleConn(conn)
	}
}

// handleConn 方法处理单个连接请求
// 它负责匹配规则、选择代理并建立连接
func (t *Tunnel) handleConn(localConn C.ServerAdapter) {
	// 函数结束时关闭本地连接
	defer localConn.Close()

	// 获取连接的目标地址
	addr := localConn.Addr()

	// 根据规则匹配合适的代理
	proxy := t.match(addr)

	// 使用选中的代理建立远程连接
	remoConn, err := proxy.Generator(addr)
	if err != nil {
		// 如果连接失败，记录警告日志
		t.logCh <- newLog(WARNING, "Proxy connect error: %s", err.Error())
		return
	}

	// 函数结束时关闭远程连接
	defer remoConn.Close()

	// 连接本地和远程连接，开始数据传输
	localConn.Connect(remoConn)
}

// match 方法根据目标地址匹配最合适的代理
// 它遍历所有规则，找到第一个匹配的规则并返回对应的代理
func (t *Tunnel) match(addr *C.Addr) C.Proxy {
	// 加读锁保护配置读取
	t.configLock.RLock()
	defer t.configLock.RUnlock()

	// 遍历所有规则
	for _, rule := range t.rules {
		// 检查规则是否匹配目标地址
		if rule.IsMatch(addr) {
			// 获取规则对应的代理
			a, ok := t.proxys[rule.Adapter()]
			if !ok {
				continue
			}
			// 记录匹配日志
			t.logCh <- newLog(INFO, "%v match %s using %s", addr.String(), rule.RuleType().String(), rule.Adapter())
			return a
		}
	}

	// 如果没有规则匹配，使用DIRECT直连代理
	t.logCh <- newLog(INFO, "%v doesn't match any rule using DIRECT", addr.String())
	return t.proxys["DIRECT"]
}

// newTunnel 创建一个新的隧道实例
func newTunnel() *Tunnel {
	// 创建日志通道
	logCh := make(chan interface{})

	// 初始化隧道结构
	tunnel := &Tunnel{
		queue:      channels.NewInfiniteChannel(),   // 创建无限缓冲通道
		proxys:     make(map[string]C.Proxy),        // 初始化代理映射
		observable: observable.NewObservable(logCh), // 创建可观察对象
		logCh:      logCh,                           // 设置日志通道
		configLock: &sync.RWMutex{},                 // 初始化读写锁
		traffic:    C.NewTraffic(time.Second),       // 初始化流量统计
	}

	// 启动处理协程
	go tunnel.process()

	// 启动日志订阅协程
	go tunnel.subscribeLogs()

	return tunnel
}

// GetInstance 获取隧道的单例实例
// 使用sync.Once确保只创建一次实例
func GetInstance() *Tunnel {
	once.Do(func() {
		tunnel = newTunnel()
	})
	return tunnel
}
