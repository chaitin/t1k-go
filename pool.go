package t1k

import "time"

// Pool 基本方法
type Pool interface {
	// 获取资源
	Get() (interface{}, error)
	// 资源放回去
	Put(interface{}) error
	// 关闭资源
	Close(interface{}) error
	// 释放所有资源
	Release()
	// 返回当前池子内有效连接数量
	Len() int
}

// PoolConfig 连接池相关配置
type PoolConfig struct {
	//连接池初始连接数
	InitialCap int
	//最大并发存活连接数
	MaxCap int
	//最大空闲连接
	MaxIdle int
	// 工厂
	Factory ConnectionFactory
	//连接最大空闲时间，超过该时间则将失效
	IdleTimeout time.Duration
}
