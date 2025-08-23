// Package observable 实现了观察者模式
// 允许对象订阅和接收数据流的通知
package observable

import (
	"errors"
	"sync"
)

// Observable 是可观察对象，实现了观察者模式的核心功能
// 它维护一个订阅者列表，当有新数据时通知所有订阅者
type Observable struct {
	iterable Iterable     // 数据源，实现了Iterable接口的数据流
	listener *sync.Map    // 订阅者映射，存储所有订阅者
	done     bool         // 是否已完成/关闭的标志
	doneLock sync.RWMutex // 读写锁，保护done状态的并发访问
}

// process 处理数据流并通知所有订阅者
// 遍历数据源中的每个项目，并将其发送给所有活跃的订阅者
func (o *Observable) process() {
	// 遍历数据源中的每个项目
	for item := range o.iterable {
		// 遍历所有订阅者并发送数据
		o.listener.Range(func(key, value interface{}) bool {
			elm := value.(*Subscriber)
			// 将项目发送给订阅者
			elm.Emit(item)
			// 继续遍历下一个订阅者
			return true
		})
	}
	// 数据流处理完成后关闭Observable
	o.close()
}

// close 关闭Observable并通知所有订阅者
// 设置done标志为true，并关闭所有订阅者的连接
func (o *Observable) close() {
	// 获取写锁以修改done状态
	o.doneLock.Lock()
	o.done = true
	o.doneLock.Unlock()

	// 遍历所有订阅者并关闭它们
	o.listener.Range(func(key, value interface{}) bool {
		elm := value.(*Subscriber)
		// 关闭订阅者连接
		elm.Close()
		// 继续遍历下一个订阅者
		return true
	})
}

// Subscribe 订阅Observable的数据流
// 返回一个新的订阅通道和可能的错误
func (o *Observable) Subscribe() (Subscription, error) {
	// 获取读锁检查Observable是否已关闭
	o.doneLock.RLock()
	done := o.done
	o.doneLock.RUnlock()

	// 如果Observable已关闭，返回错误
	if done == true {
		return nil, errors.New("Observable is closed")
	}

	// 创建新的订阅者
	subscriber := newSubscriber()

	// 将订阅者存储到监听器映射中
	o.listener.Store(subscriber.Out(), subscriber)

	// 返回订阅者的输出通道
	return subscriber.Out(), nil
}

// UnSubscribe 取消订阅指定的订阅通道
// 从监听器列表中移除订阅者并关闭它
func (o *Observable) UnSubscribe(sub Subscription) {
	// 从监听器映射中查找订阅者
	elm, exist := o.listener.Load(sub)
	if !exist {
		// 如果订阅者不存在，打印提示信息并返回
		println("not exist")
		return
	}

	// 获取订阅者实例
	subscriber := elm.(*Subscriber)

	// 从监听器映射中删除订阅者
	o.listener.Delete(subscriber.Out())

	// 关闭订阅者
	subscriber.Close()
}

// NewObservable 创建一个新的Observable实例
// any: 实现了Iterable接口的数据源
// 返回: 配置好的Observable实例
func NewObservable(any Iterable) *Observable {
	// 创建Observable实例
	observable := &Observable{
		iterable: any,         // 设置数据源
		listener: &sync.Map{}, // 初始化订阅者映射
	}

	// 在单独的goroutine中启动处理过程
	go observable.process()

	// 返回Observable实例
	return observable
}
