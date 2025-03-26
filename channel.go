package t1k

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/chaitin/t1k-go/detection"
	"github.com/chaitin/t1k-go/misc"
	"github.com/chaitin/t1k-go/t1k"
)

// ChannelPool 存放连接信息
type ChannelPool struct {
	mu                       sync.RWMutex
	conns                    chan *idleConn    // 存储最大空闲连接
	factory                  ConnectionFactory // 工厂
	idleTimeout, waitTimeOut time.Duration     // 连接空闲超时和等待超时
	maxActive                int               // 最大连接数
	openingConns             int               // 活跃的连接数
	connReqs                 []chan connReq    // 连接请求缓冲区，如果无法从 conns 取到连接，则在这个缓冲区创建一个新的元素，之后连接放回去时先填充这个缓冲区
}

type idleConn struct {
	conn interface{}
	t    time.Time
}

type connReq struct {
	idleConn *idleConn
}

// NewChannelPool 初始化连接
func NewChannelPool(poolConfig *PoolConfig) (*ChannelPool, error) {
	// 校验参数
	if !(poolConfig.InitialCap <= poolConfig.MaxIdle && poolConfig.MaxCap >= poolConfig.MaxIdle && poolConfig.InitialCap >= 0) {
		return nil, errors.New("invalid capacity settings")
	}
	// 校验参数
	if poolConfig.Factory == nil {
		return nil, errors.New("invalid factory interface settings")
	}

	c := &ChannelPool{
		conns:        make(chan *idleConn, poolConfig.MaxIdle), // 最大空闲连接数
		factory:      poolConfig.Factory,
		idleTimeout:  poolConfig.IdleTimeout,
		maxActive:    poolConfig.MaxCap,
		openingConns: poolConfig.InitialCap,
	}

	// 初始化初始连接放入 channel 中
	for i := 0; i < poolConfig.InitialCap; i++ {
		conn, err := c.factory.Factory()
		if err != nil {
			c.Release()
			return nil, fmt.Errorf("factory is not able to fill the pool: %s", err)
		}
		c.conns <- &idleConn{conn: conn, t: time.Now()}
	}

	return c, nil
}

// GetConns 获取所有连接
func (c *ChannelPool) getConns() chan *idleConn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conns
}

// Get 从pool中取一个连接
func (c *ChannelPool) Get() (interface{}, error) {
	ErrClosed := errors.New("pool is closed")
	ErrMaxActiveConnReached := errors.New("max active connections reached")

	conns := c.getConns()
	if conns == nil {
		return nil, ErrClosed
	}
	for {
		select {
		// 优先从空闲连接缓冲取
		case wrapConn := <-conns:
			if wrapConn == nil {
				return nil, ErrClosed
			}
			//判断是否超时，超时则丢弃
			if timeout := c.idleTimeout; timeout > 0 {
				if wrapConn.t.Add(timeout).Before(time.Now()) {
					//丢弃并关闭该连接
					_ = c.Close(wrapConn.conn)
					continue
				}
			}
			//判断是否失效，失效则丢弃，如果用户没有设定 ping 方法，就不检查
			if err := c.Ping(wrapConn.conn); err != nil {
				_ = c.Close(wrapConn.conn)
				continue
			}
			return wrapConn.conn, nil
		default:
			// 没有空闲连接
			c.mu.Lock()
			// log.Printf("openConn %v %v", c.openingConns, c.maxActive)
			// 判断连接数是否达到上限
			if c.openingConns >= c.maxActive {
				req := make(chan connReq, 1)
				// 如果达到上限，则创建一个缓冲位置，等待放回去的连接
				c.connReqs = append(c.connReqs, req)
				c.mu.Unlock()
				// 判断是否有连接放回去（放回去逻辑在 put 方法内）
				ret, ok := <-req
				// 如果没有连接放回去，则不能再创建新的连接了，因为达到上限了
				if !ok {
					return nil, ErrMaxActiveConnReached
				}
				// 如果有连接放回去了 判断连接是否可用
				if timeout := c.idleTimeout; timeout > 0 {
					if ret.idleConn.t.Add(timeout).Before(time.Now()) {
						// 丢弃并关闭该连接
						// 重新尝试获取连接
						_ = c.Close(ret.idleConn.conn)
						continue
					}
				}
				return ret.idleConn.conn, nil
			}
			// 到这里说明 没有空闲连接 && 连接数没有达到上限 可以创建新连接
			if c.factory == nil {
				c.mu.Unlock()
				return nil, ErrClosed
			}
			conn, err := c.factory.Factory()
			if err != nil {
				c.mu.Unlock()
				return nil, err
			}
			// 连接数+1
			c.openingConns++
			c.mu.Unlock()
			return conn, nil
		}
	}
}

// Put 将连接放回pool中
func (c *ChannelPool) Put(conn interface{}) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}

	c.mu.Lock()

	// 检查连接池是否已被释放
	if c.conns == nil || c.factory == nil {
		c.mu.Unlock()
		// 如果 factory 为 nil，直接返回，避免调用 Close
		if c.factory == nil {
			return errors.New("connection pool has been released")
		}
		return c.Close(conn)
	}

	// 如果有请求连接的缓冲区有等待，则按顺序有限个先来的请求分配当前放回的连接
	if l := len(c.connReqs); l > 0 {
		req := c.connReqs[0]
		copy(c.connReqs, c.connReqs[1:])
		c.connReqs = c.connReqs[:l-1]
		req <- connReq{
			idleConn: &idleConn{conn: conn, t: time.Now()},
		}
		c.mu.Unlock()
		return nil
	}

	// 如果没有等待的缓冲则尝试放入空闲连接缓冲
	select {
	case c.conns <- &idleConn{conn: conn, t: time.Now()}:
		c.mu.Unlock()
		return nil
	default:
		//连接池已满，直接关闭该连接
		// log.Printf("connection pool is full, close this connection")
		c.mu.Unlock()
		return c.Close(conn)
	}
}

// Close 关闭单条连接
func (c *ChannelPool) Close(conn interface{}) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// 连接数减一
	c.openingConns--
	// 增加对 factory 的空检查
	if c.factory == nil {
		return errors.New("factory is nil, connection pool might be released")
	}
	// 调用工厂的关闭方法
	return c.factory.Close(conn)
}

// Release 释放连接池中所有连接
func (c *ChannelPool) Release() {
	c.mu.Lock()
	conns := c.conns
	c.conns = nil
	c.mu.Unlock()

	defer func() {
		c.factory = nil
	}()

	if conns == nil {
		return
	}

	close(conns)
	for wrapConn := range conns {
		// log.Printf("Type %v\n",reflect.TypeOf(wrapConn.conn))
		_ = c.factory.Close(wrapConn.conn)
	}
}

// Ping 检查单条连接是否有效
func (c *ChannelPool) Ping(conn interface{}) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}

	return c.factory.Ping(conn)
}

// Len 连接池中已有的连接
func (c *ChannelPool) Len() int {
	return len(c.getConns())
}

// DetectHttpRequest 检测 HTTP 请求
func (c *ChannelPool) DetectHttpRequest(req *http.Request) (*detection.Result, error) {
	conn, err := c.Get()
	if err != nil {
		return nil, err
	}
	defer c.Put(conn)
	tcpConn, ok := conn.(net.Conn)
	if !ok {
		return nil, errors.New("invalid connection type")
	}
	return DetectHttpRequest(tcpConn, req)
}

////////////////////
// 以下为自定义方法 //
////////////////////

// DetectRequest 自定义检测请求
func (c *ChannelPool) DetectRequest(req detection.Request) (*detection.Result, error) {
	conn, err := c.Get()
	if err != nil {
		return nil, err
	}
	defer c.Put(conn)
	tcpConn, ok := conn.(net.Conn)
	if !ok {
		return nil, errors.New("invalid connection type")
	}
	return DetectRequest(tcpConn, req)
}

// DetectRequestInCtx 自定义上下文检测请求
func (c *ChannelPool) DetectRequestInCtx(dc *detection.DetectionContext) (*detection.Result, error) {
	conn, err := c.Get()
	if err != nil {
		return nil, err
	}
	defer c.Put(conn)
	tcpConn, ok := conn.(net.Conn)
	if !ok {
		return nil, errors.New("invalid connection type")
	}
	return DetectRequestInCtx(tcpConn, dc)
}

// DetectResponseInCtx 自定义上下文检测响应
func (c *ChannelPool) DetectResponseInCtx(dc *detection.DetectionContext) (*detection.Result, error) {
	conn, err := c.Get()
	if err != nil {
		return nil, err
	}
	defer c.Put(conn)
	tcpConn, ok := conn.(net.Conn)
	if !ok {
		return nil, errors.New("invalid connection type")
	}
	return DetectResponseInCtx(tcpConn, dc)
}

// WriteSection 自定义写入 Section
func (c *ChannelPool) WriteSection(sec t1k.Section) error {
	conn, err := c.Get()
	if err != nil {
		return err
	}
	defer c.Put(conn)
	err = t1k.WriteSection(sec, conn.(net.Conn))
	return misc.ErrorWrap(err, "")
}

// ReadSection 自定义读取 Section
func (c *ChannelPool) ReadSection() (t1k.Section, error) {
	conn, err := c.Get()
	if err != nil {
		return nil, err
	}
	defer c.Put(conn)
	sec, err := t1k.ReadSection(conn.(net.Conn))
	if err != nil {
		return nil, misc.ErrorWrap(err, "")
	}
	return sec, nil
}

// ReadFullSection 自定义读取 Section
func (c *ChannelPool) ReadFullSection() (t1k.Section, error) {
	conn, err := c.Get()
	if err != nil {
		return nil, err
	}
	defer c.Put(conn)
	sec, err := t1k.ReadFullSection(conn.(net.Conn))
	if err != nil {
		return nil, misc.ErrorWrap(err, "")
	}
	return sec, nil
}
