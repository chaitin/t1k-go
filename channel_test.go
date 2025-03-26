package t1k

import (
	"net"
	"testing"
	"time"
)

// 创建一个模拟TcpFactory
type MockFactory struct{}

func (f *MockFactory) Factory() (interface{}, error) {
	// 创建一对内存中相互连接的连接
	client, _ := net.Pipe()
	return client, nil
}

func (f *MockFactory) Close(v interface{}) error {
	return nil
}

func (f *MockFactory) Ping(v interface{}) error {
	return nil
}

func TestPutAfterRelease(t *testing.T) {
	// 创建连接池
	pool, _ := NewChannelPool(&PoolConfig{
		InitialCap:  1,
		MaxIdle:     16,
		MaxCap:      32,
		Factory:     &MockFactory{},
		IdleTimeout: 30 * time.Second},
	)

	// 获取一个连接
	conn, _ := pool.Get()

	// 释放连接池
	pool.Release()

	// 尝试归还连接，应该不会panic，而是返回一个错误
	err := pool.Put(conn)
	if err == nil {
		t.Error("Expected error when putting connection after release")
	}
}
