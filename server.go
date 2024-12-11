package t1k

import (
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/chaitin/t1k-go/detection"

	"github.com/chaitin/t1k-go/misc"
)

const (
	DEFAULT_POOL_SIZE  = 8
	HEARTBEAT_INTERVAL = 20
)

type Server struct {
	socketFactory func() (net.Conn, error)
	poolCh        chan *conn
	poolSize      int
	count         int
	closeCh       chan struct{}
	logger        *log.Logger
	mu            sync.Mutex

	healthCheck *HealthCheckService
}

func (s *Server) newConn() error {
	sock, err := s.socketFactory()
	if err != nil {
		return err
	}
	s.count += 1
	s.poolCh <- makeConn(sock, s)
	return nil
}

func (s *Server) GetConn() (*conn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.count < s.poolSize {
		for i := 0; i < (s.poolSize - s.count); i++ {
			err := s.newConn()
			if err != nil {
				return nil, err
			}
		}
	}

	select {
	case c, ok := <-s.poolCh:
		if !ok {
			return nil, errors.New("connection pool closed")
		}
		return c, nil
	default:
		return nil, errors.New("no available connections")
	}
}

func (s *Server) PutConn(c *conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.failing {
		s.count -= 1
		c.Close()
	} else {
		s.poolCh <- c
	}
}

func (s *Server) broadcastHeartbeat() {
	l := len(s.poolCh)
	for i := 0; i < l; i++ {
		select {
		case c := <-s.poolCh:
			c.Heartbeat()
			s.PutConn(c)
		default:
			return
		}
	}
}

func (s *Server) runHeartbeatCo() {
	interval := HEARTBEAT_INTERVAL
	intervalRaw := os.Getenv("T1K_HEARTBEAT_INTERVAL")
	if intervalRaw != "" {
		val, err := strconv.Atoi(intervalRaw)
		if err == nil {
			interval = val
		}
	}
	for {
		timer := time.NewTimer(time.Duration(interval) * time.Second)
		select {
		case <-s.closeCh:
			timer.Stop()
			return
		case <-timer.C:
		}
		s.broadcastHeartbeat()
	}
}

func (s *Server) UpdateHealthCheckConfig(config *HealthCheckConfig) error {
	return s.healthCheck.UpdateConfig(config)
}

func (s *Server) IsHealth() bool {
	return s.healthCheck.IsHealth()
}

func (s *Server) HealthCheckStats() HealthCheckStats {
	stats := s.healthCheck.HealthCheckStats()
	return stats
}

func NewFromSocketFactoryWithPoolSize(socketFactory func() (net.Conn, error), poolSize int) (*Server, error) {
	ret := &Server{
		socketFactory: socketFactory,
		poolCh:        make(chan *conn, poolSize),
		poolSize:      poolSize,
		closeCh:       make(chan struct{}),
		logger:        log.New(os.Stdout, "snserver", log.LstdFlags),
		mu:            sync.Mutex{},
	}
	for i := 0; i < poolSize; i++ {
		err := ret.newConn()
		if err != nil {
			return nil, err
		}
	}
	healthCheck, err := NewHealthCheckService()
	if err != nil {
		return nil, err
	}
	ret.healthCheck = healthCheck

	go ret.runHeartbeatCo()
	go ret.healthCheck.Run() //FIXME: health check endless loop
	return ret, nil
}

func NewFromSocketFactory(socketFactory func() (net.Conn, error)) (*Server, error) {
	return NewFromSocketFactoryWithPoolSize(socketFactory, DEFAULT_POOL_SIZE)
}

func NewWithPoolSize(addr string, poolSize int) (*Server, error) {
	return NewFromSocketFactoryWithPoolSize(func() (net.Conn, error) {
		return net.Dial("tcp", addr)
	}, poolSize)
}

func New(addr string) (*Server, error) {
	return NewWithPoolSize(addr, DEFAULT_POOL_SIZE)
}

func (s *Server) DetectRequestInCtx(dc *detection.DetectionContext) (*detection.Result, error) {
	c, err := s.GetConn()
	if err != nil {
		return nil, err
	}
	defer s.PutConn(c)
	return c.DetectRequestInCtx(dc)
}

func (s *Server) DetectResponseInCtx(dc *detection.DetectionContext) (*detection.Result, error) {
	c, err := s.GetConn()
	if err != nil {
		return nil, misc.ErrorWrap(err, "")
	}
	defer s.PutConn(c)
	return c.DetectResponseInCtx(dc)
}

func (s *Server) Detect(dc *detection.DetectionContext) (*detection.Result, *detection.Result, error) {
	c, err := s.GetConn()
	if err != nil {
		return nil, nil, misc.ErrorWrap(err, "")
	}

	reqResult, rspResult, err := c.Detect(dc)
	if err == nil {
		s.PutConn(c)
	}
	return reqResult, rspResult, err
}

func (s *Server) DetectHttpRequest(req *http.Request) (*detection.Result, error) {
	c, err := s.GetConn()
	if err != nil {
		return nil, err
	}
	defer s.PutConn(c)
	return c.DetectHttpRequest(req)
}

func (s *Server) DetectRequest(req detection.Request) (*detection.Result, error) {
	c, err := s.GetConn()
	if err != nil {
		return nil, err
	}
	defer s.PutConn(c)
	return c.DetectRequest(req)
}

// blocks until all pending detection is completed
func (s *Server) Close() {
	close(s.closeCh)
	for i := 0; i < s.count; i++ {
		c, err := s.GetConn()
		if err != nil {
			return
		}
		c.Close()
	}
	s.healthCheck.Close()
}
