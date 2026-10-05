package agent

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type monitorConfig struct {
	cpuLimit  float64
	ramLimit  float64
	diskLimit float64
	interval  time.Duration
	diskPath  string
}

func loadMonitorConfig() (monitorConfig, error) {
	cfg := monitorConfig{interval: 30 * time.Second, diskPath: "C:\\"}
	var err error
	cfg.cpuLimit, err = percentFromEnv("CPU_LIMIT", 90)
	if err != nil {
		return monitorConfig{}, err
	}
	cfg.ramLimit, err = percentFromEnv("RAM_LIMIT", 75)
	if err != nil {
		return monitorConfig{}, err
	}
	cfg.diskLimit, err = percentFromEnv("DISK_LIMIT", 90)
	if err != nil {
		return monitorConfig{}, err
	}
	if value := strings.TrimSpace(os.Getenv("MONITOR_INTERVAL")); value != "" {
		cfg.interval, err = time.ParseDuration(value)
		if err != nil || cfg.interval <= 0 {
			return monitorConfig{}, fmt.Errorf("MONITOR_INTERVAL deve ser uma duração positiva, como 30s ou 1m")
		}
	}
	if value := strings.TrimSpace(os.Getenv("DISK_PATH")); value != "" {
		if !filepath.IsAbs(value) {
			return monitorConfig{}, fmt.Errorf("DISK_PATH deve ser um caminho absoluto, como C:\\")
		}
		cfg.diskPath = value
	}
	return cfg, nil
}

func percentFromEnv(name string, fallback float64) (float64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	percent, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 0 || percent > 100 {
		return 0, fmt.Errorf("%s deve ser um número entre 0 e 100; use ponto para decimais", name)
	}
	return percent, nil
}
