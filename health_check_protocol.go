package t1k

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	HEALTH_CHECK_T1K_PROTOCOL  = "t1k"
	HEALTH_CHECK_HTTP_PROTOCOL = "http"
)

type HCProtocol interface {
	Check() (bool, string)
}

type T1KProtocol struct {
	Addresses []string
	Timeout   int64 // Millisecond
}

type T1kHealthCheckResult struct {
	OK     bool
	Server string
	Info   string
}

func (t1kProto *T1KProtocol) checkSingle(ctx context.Context, address string, results chan T1kHealthCheckResult) {
	result := T1kHealthCheckResult{}

	conn, err := net.Dial("tcp", address)
	if err != nil {
		result.OK = false
		result.Info = err.Error()
		return
	}
	defer conn.Close()
	result.Server = address

	err = DoHeartbeat(conn)
	if err != nil {
		result.OK = false
		result.Info = err.Error()
	} else {
		result.OK = true
	}

	select {
	case <-ctx.Done():
		result.OK = false
		result.Info = "health check timeout"
		results <- result
	case results <- result:
	}
}

func (t1kProto *T1KProtocol) Check() (bool, string) {
	addressesNum := len(t1kProto.Addresses)
	if addressesNum == 0 {
		return false, "not available address"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*time.Duration(t1kProto.Timeout))
	defer cancel()

	results := make(chan T1kHealthCheckResult, addressesNum)
	for i := 0; i < addressesNum; i++ {
		go t1kProto.checkSingle(ctx, t1kProto.Addresses[i], results)
	}

	successNum := 0
	for {
		select {
		case <-ctx.Done():
			return false, "health check timeout"
		case result := <-results:
			if result.OK {
				successNum++
				if successNum == addressesNum {
					return true, ""
				}
			} else {
				return false, fmt.Sprintf("server %s health check error: %s", result.Server, result.Info)
			}
		}
	}
}

func NewT1KProtocol(addresses []string, timeout int64) *T1KProtocol {
	return &T1KProtocol{
		Addresses: addresses,
		Timeout:   timeout,
	}
}

type HTTPProtocol struct {
	Addresses []string
	Timeout   int64 // Millisecond
}

type HTTPHealthCheckResult struct {
	OK     bool
	Server string
	Info   string
}

func (httpProto *HTTPProtocol) checkSingle(ctx context.Context, address string, results chan HTTPHealthCheckResult) {
	result := HTTPHealthCheckResult{}
	resp, err := http.Get(address)
	if err != nil {
		result.OK = false
		result.Info = err.Error()
		return
	}
	defer resp.Body.Close()
	result.Server = address
	if resp.StatusCode != http.StatusOK {
		result.OK = false
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			result.Info = "response code is not 200 and cannot get result."
		}
		result.Info = string(body[:])
	} else {
		result.OK = true
	}

	select {
	case <-ctx.Done():
		result.OK = false
		result.Info = fmt.Sprintf("health check timeout for %s", address)
		results <- result
	case results <- result:
	}
}

func (httpProto *HTTPProtocol) Check() (bool, string) {
	addressesNum := len(httpProto.Addresses)
	if addressesNum == 0 {
		return false, "not available address"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*time.Duration(httpProto.Timeout))
	defer cancel()

	results := make(chan HTTPHealthCheckResult, addressesNum)
	for i := 0; i < addressesNum; i++ {
		go httpProto.checkSingle(ctx, httpProto.Addresses[i], results)
	}

	successNum := 0
	for {
		select {
		case <-ctx.Done():
			return false, "health check timeout"
		case result := <-results:
			if result.OK {
				successNum++
				if successNum == addressesNum {
					return true, ""
				}
			} else {
				return false, fmt.Sprintf("server %s health check error: %s", result.Server, result.Info)
			}
		}
	}
}

func NewHTTPProtocol(addresses []string, timeout int64, enableTLS bool) *HTTPProtocol {
	healthCheckURL := []string{}
	for _, address := range addresses {
		scheme := "http"
		if enableTLS {
			scheme = "https"
		}
		urlPath := fmt.Sprintf("%s://%s/stat", scheme, address)
		healthCheckURL = append(healthCheckURL, urlPath)
	}
	return &HTTPProtocol{
		Addresses: healthCheckURL,
		Timeout:   timeout,
	}
}
