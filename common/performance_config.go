package common

import "sync/atomic"

// PerformanceMonitorConfig 性能监控配置
type PerformanceMonitorConfig struct {
	Enabled         bool
	CPUThreshold    int
	MemoryThreshold int
	DiskThreshold   int
}

var performanceMonitorConfig atomic.Value

func init() {
	// 初始化默认配置 (DiskThreshold 设为 0 禁用磁盘熔断拦截，避免因 Windows 宿主盘容量报警导致误杀)
	performanceMonitorConfig.Store(PerformanceMonitorConfig{
		Enabled:         true,
		CPUThreshold:    90,
		MemoryThreshold: 90,
		DiskThreshold:   0,
	})
}

// GetPerformanceMonitorConfig 获取性能监控配置
func GetPerformanceMonitorConfig() PerformanceMonitorConfig {
	return performanceMonitorConfig.Load().(PerformanceMonitorConfig)
}

// SetPerformanceMonitorConfig 设置性能监控配置
func SetPerformanceMonitorConfig(config PerformanceMonitorConfig) {
	performanceMonitorConfig.Store(config)
}
