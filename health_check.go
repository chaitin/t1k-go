package t1k

import (
	"time"
)

type HealthCheckConfig struct {
	Interval            int64    // default 1s
	HealthThreshold     int64    // default 5
	UnhealthThreshold   int64    // default 3
	Addresses           []string // like ['1.1.1.1:80', '1.1.1.2:8000']
	Timeout             int64    // default 3000 millisecond
	HealthCheckProtocol string
	EnableTLS           bool
}

type HealthCheckStats struct {
	Count           uint64
	ErrorCount      int64
	Panic           bool
	LatestErrorInfo string
	Status          string
}

type HealthCheckService struct {
	healthCheckConfig *HealthCheckConfig
	Stats             *HealthCheckStats
	exitChan          chan bool
	configChan        chan *HealthCheckConfig
}

const (
	HealthCheckRunningStatus = "running"
	HealthCheckStoppedStatus = "stopped"
)

// IsHealth return  health check result
func (hcs *HealthCheckService) IsHealth() bool {
	if hcs.Stats.ErrorCount > hcs.healthCheckConfig.UnhealthThreshold {
		return false
	}
	// from unhealth to health
	if hcs.Stats.ErrorCount < 0 {
		return false
	}
	if hcs.Stats.Panic {
		return false
	}
	return true
}

// HealthDetailInfo return health check result with detail info
func (hcs *HealthCheckService) HealthDetailInfo() string {
	return hcs.Stats.LatestErrorInfo
}

// HealthCheckStats return health check stats
func (hcs *HealthCheckService) HealthCheckStats() HealthCheckStats {
	return *hcs.Stats
}

// UpdateConfig trigger the health check or update health check config
func (hcs *HealthCheckService) UpdateConfig(config *HealthCheckConfig) error {
	healthCheck := &HealthCheckConfig{}
	if config.Interval <= 0 {
		healthCheck.Interval = 1
	} else {
		healthCheck.Interval = config.Interval
	}

	if config.UnhealthThreshold <= 0 {
		healthCheck.UnhealthThreshold = 3
	} else {
		healthCheck.UnhealthThreshold = config.UnhealthThreshold
	}

	if config.HealthThreshold <= 0 {
		healthCheck.HealthThreshold = 5
	} else {
		healthCheck.HealthThreshold = config.HealthThreshold
	}

	if config.Timeout <= 0 {
		healthCheck.Timeout = 3000
	}

	healthCheck.Addresses = config.Addresses
	healthCheck.HealthCheckProtocol = config.HealthCheckProtocol
	healthCheck.EnableTLS = config.EnableTLS
	hcs.configChan <- healthCheck
	return nil
}

// current support t1k protocol
func (hcs *HealthCheckService) GetHealthCheckProtocol() string {
	return hcs.healthCheckConfig.HealthCheckProtocol
}

func (hcs *HealthCheckService) CaclErrorCount(ok bool, info string) {
	//         unhealth    |    health    unhealth
	// ____________________0____________x______________->
	//-                           UnhealthThreshold     +
	// ErrorCount == 0 is health
	// 0 < ErrorCount <= UnhealthThreshold is health
	// ErrorCount > UnhealthThreshold is unhealth
	// ErrorCount < 0 is unhealth

	if !ok {
		// already in unhealth, reset
		if hcs.Stats.ErrorCount < 0 {
			hcs.Stats.ErrorCount = -hcs.healthCheckConfig.HealthThreshold
		} else {
			hcs.Stats.ErrorCount += 1
			hcs.Stats.LatestErrorInfo = info
			if hcs.Stats.ErrorCount > hcs.healthCheckConfig.UnhealthThreshold {
				// in unhealth
				hcs.Stats.ErrorCount = -hcs.healthCheckConfig.HealthThreshold
			}
		}
	} else {
		// from unhealth to health
		if hcs.Stats.ErrorCount < 0 {
			hcs.Stats.ErrorCount += 1
		} else if hcs.Stats.ErrorCount-1 < 0 { // health
			hcs.Stats.ErrorCount = 0
		} else {
			hcs.Stats.ErrorCount = 0 //health, reset
		}
		hcs.Stats.LatestErrorInfo = ""
	}
}

func (hcs *HealthCheckService) ClearStats() {
	hcs.Stats.Count = 0
	hcs.Stats.ErrorCount = 0
	hcs.Stats.LatestErrorInfo = ""
	hcs.Stats.Panic = false
	hcs.Stats.Status = HealthCheckStoppedStatus
}

// Run start a health check go routine.
// If you want health check enable, need invoke UpdateConfig to trigger.
func (hcs *HealthCheckService) Run() error {
	defer func() {
		if r := recover(); r != nil {
			// panic need rerun NewHealthCheckService to recover
			hcs.Stats.Panic = true
			hcs.Stats.Status = HealthCheckStoppedStatus
		}
	}()

	for {
		config := <-hcs.configChan
		if hcs.healthCheckConfig != nil {
			// only single Run instance
			return nil
		}
		hcs.healthCheckConfig = config
		if hcs.healthCheckConfig != nil {
			break
		}
	}
rerun:

	hcs.ClearStats()
	hcs.Stats.Status = HealthCheckRunningStatus

	// init protocol instance
	var protocolIns HCProtocol
	switch hcs.healthCheckConfig.HealthCheckProtocol {
	case HEALTH_CHECK_T1K_PROTOCOL:
		protocolIns = NewT1KProtocol(hcs.healthCheckConfig.Addresses, hcs.healthCheckConfig.Timeout)
	case HEALTH_CHECK_HTTP_PROTOCOL:
		protocolIns = NewHTTPProtocol(hcs.healthCheckConfig.Addresses, hcs.healthCheckConfig.Timeout, hcs.healthCheckConfig.EnableTLS)
	default:
		protocolIns = NewT1KProtocol(hcs.healthCheckConfig.Addresses, hcs.healthCheckConfig.Timeout)
	}

	tricker := time.NewTicker(time.Duration(hcs.healthCheckConfig.Interval) * time.Second)
	for {
		select {
		case <-tricker.C:
			hcs.Stats.Count += 1
			ok, info := protocolIns.Check()
			hcs.CaclErrorCount(ok, info)
		case config := <-hcs.configChan:
			hcs.healthCheckConfig = config
			goto rerun
		case <-hcs.exitChan:
			hcs.ClearStats()
			return nil
		}
	}
}

func (hcs *HealthCheckService) Close() {
	hcs.exitChan <- true
	close(hcs.exitChan)

	close(hcs.configChan)
}

// NewHealthCheckService create new HealthCheckService for health check.
// After create new health check service, invoke UpdateConfig to update health check
func NewHealthCheckService() (*HealthCheckService, error) {
	healthCheckStats := &HealthCheckStats{}

	healthCheckService := &HealthCheckService{
		Stats:      healthCheckStats,
		configChan: make(chan *HealthCheckConfig, 1),
		exitChan:   make(chan bool, 1),
	}
	return healthCheckService, nil
}
