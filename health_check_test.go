package t1k

import (
	"errors"
	"testing"
)

func InitHealthCheckIns(healthCheckConfig *HealthCheckConfig) (*HealthCheckService, error) {
	hcs, err := NewHealthCheckService()
	if err != nil {
		return nil, errors.New(err.Error())
	}

	hcs.healthCheckConfig = healthCheckConfig
	return hcs, nil
}

func TestCaclErrorCountHealth(t *testing.T) {

	// Interval: 1
	// UnhealthThreshold: 3
	// HealthThreshold: 5
	// Timeout: 3000
	healthCheckConfig := &HealthCheckConfig{
		Interval:          1,
		UnhealthThreshold: 3,
		HealthThreshold:   5,
		Timeout:           3000,
		Addresses:         []string{"1.1.1.1:8000"},
	}

	hcs, err := InitHealthCheckIns(healthCheckConfig)
	if err != nil {
		t.Errorf("InitHealthCheckIns error: %s", err.Error())
	}

	count := 100000
	for i := 0; i < count; i++ {
		hcs.CaclErrorCount(true, "")
	}

	if int(hcs.HealthCheckStats().ErrorCount) != 0 {
		t.Errorf("health check ErrorCount incorrect")
	}

	if !hcs.IsHealth() {
		t.Errorf("IsHealth incorrect")
	}
}

func TestCaclErrorCountUnHealth(t *testing.T) {

	// Interval: 1
	// UnhealthThreshold: 3
	// HealthThreshold: 5
	// Timeout: 3000
	healthCheckConfig := &HealthCheckConfig{
		Interval:          1,
		UnhealthThreshold: 3,
		HealthThreshold:   5,
		Timeout:           3000,
		Addresses:         []string{"1.1.1.1:8000"},
	}

	hcs, err := InitHealthCheckIns(healthCheckConfig)
	if err != nil {
		t.Errorf("InitHealthCheckIns error: %s", err.Error())
	}

	count := int64(100000)
	for i := int64(0); i < count; i++ {
		hcs.CaclErrorCount(false, "")
	}

	if hcs.HealthCheckStats().ErrorCount != -hcs.healthCheckConfig.HealthThreshold {
		t.Errorf("health check ErrorCount incorrect")
	}

	if hcs.IsHealth() {
		t.Errorf("IsHealth incorrect")
	}
}

func TestCaclErrorCount(t *testing.T) {
	// Interval: 1
	// UnhealthThreshold: 3
	// HealthThreshold: 5
	// Timeout: 3000
	healthCheckConfig := &HealthCheckConfig{
		Interval:          1,
		UnhealthThreshold: 3,
		HealthThreshold:   5,
		Timeout:           3000,
		Addresses:         []string{"1.1.1.1:8000"},
	}

	hcs, err := InitHealthCheckIns(healthCheckConfig)
	if err != nil {
		t.Errorf("InitHealthCheckIns error: %s", err.Error())
	}

	// ErrorCount is 2
	errorCount := int64(2)
	for i := int64(0); i < errorCount; i++ {
		hcs.CaclErrorCount(false, "error")
	}
	if hcs.HealthCheckStats().ErrorCount != errorCount {
		t.Errorf("ErrorCount incorrect")
	}
	if !hcs.IsHealth() {
		t.Errorf("IsHealth incorrect")
	}

	// ErrorCount is 0
	hcs.CaclErrorCount(true, "")
	if hcs.HealthCheckStats().ErrorCount != 0 {
		t.Errorf("ErrorCount incorrect")
	}
	if !hcs.IsHealth() {
		t.Errorf("IsHealth incorrect")
	}

	// ErrorCount is 3
	errorCount = int64(3)
	for i := int64(0); i < errorCount; i++ {
		hcs.CaclErrorCount(false, "error")
	}
	if hcs.HealthCheckStats().ErrorCount != errorCount {
		t.Errorf("ErrorCount incorrect")
	}
	if !hcs.IsHealth() {
		t.Errorf("IsHealth incorrect")
	}

	// ErrorCount is -5
	hcs.CaclErrorCount(false, "")
	if hcs.HealthCheckStats().ErrorCount != -hcs.healthCheckConfig.HealthThreshold {
		t.Errorf("ErrorCount incorrect")
	}
	if hcs.IsHealth() {
		t.Errorf("IsHealth incorrect")
	}

	// ErrorCount is -3
	successCount := int64(2)
	for i := int64(0); i < successCount; i++ {
		hcs.CaclErrorCount(true, "")
	}
	if hcs.HealthCheckStats().ErrorCount != -hcs.healthCheckConfig.HealthThreshold+successCount {
		t.Errorf("ErrorCount incorrect")
	}
	if hcs.IsHealth() {
		t.Errorf("IsHealth incorrect")
	}

	// ErrorCount is -5
	hcs.CaclErrorCount(false, "")
	if hcs.HealthCheckStats().ErrorCount != -hcs.healthCheckConfig.HealthThreshold {
		t.Errorf("ErrorCount incorrect")
	}
	if hcs.IsHealth() {
		t.Errorf("IsHealth incorrect")
	}

	// ErrorCount is 0
	successCount = int64(hcs.healthCheckConfig.HealthThreshold)
	for i := int64(0); i < successCount; i++ {
		hcs.CaclErrorCount(true, "")
	}
	if hcs.HealthCheckStats().ErrorCount != 0 {
		t.Errorf("ErrorCount incorrect")
	}
	if !hcs.IsHealth() {
		t.Errorf("IsHealth incorrect")
	}

}
