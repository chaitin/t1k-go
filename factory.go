package t1k

import (
	"errors"
	"net"
	"time"
)

// ConnectionFactory 连接工厂
type ConnectionFactory interface {
	//生成连接的方法
	Factory() (interface{}, error)
	//关闭连接的方法
	Close(interface{}) error
	//检查连接是否有效的方法
	Ping(interface{}) error
}

// TcpFactory 结构体
type TcpFactory struct {
	Addr string
}

// Factory 方法生成 TCP 连接
func (t *TcpFactory) Factory() (interface{}, error) {
	conn, err := net.DialTimeout("tcp", t.Addr, 3*time.Second)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// Close 方法关闭 TCP 连接
func (t *TcpFactory) Close(conn interface{}) error {
	tcpConn, ok := conn.(net.Conn)
	if !ok {
		return errors.New("invalid connection type")
	}
	return tcpConn.Close()
}

// Ping 方法检查 TCP 连接是否有效
func (f *TcpFactory) Ping(conn interface{}) error {
	tcpConn, ok := conn.(net.Conn)
	if !ok {
		return errors.New("invalid connection type")
	}
	// // 发送一个空的 TCP 数据包来检查连接是否有效
	// if err := tcpConn.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
	// 	return err
	// }
	// if _, err := tcpConn.Write([]byte{}); err != nil {
	// 	return err
	// }
	// return tcpConn.SetDeadline(time.Time{})
	err := DoHeartbeat(tcpConn)
	return err
}
