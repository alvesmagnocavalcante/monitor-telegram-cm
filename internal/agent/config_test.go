package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func clearMonitorEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"CPU_LIMIT", "RAM_LIMIT", "DISK_LIMIT", "MONITOR_INTERVAL", "DISK_PATH"} {
		t.Setenv(key, "")
	}
}

func TestMonitorConfigDefaults(t *testing.T) {
	clearMonitorEnv(t)
	cfg, err := loadMonitorConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.cpuLimit != 90 || cfg.ramLimit != 75 || cfg.diskLimit != 90 || cfg.interval != 30*time.Second || cfg.diskPath != "C:\\" {
		t.Fatalf("valores padrão incorretos: %+v", cfg)
	}
}

func TestMonitorConfigOverrides(t *testing.T) {
	clearMonitorEnv(t)
	disk := t.TempDir()
	t.Setenv("CPU_LIMIT", "60.5")
	t.Setenv("RAM_LIMIT", " 75 ")
	t.Setenv("DISK_LIMIT", "85")
	t.Setenv("MONITOR_INTERVAL", "1m")
	t.Setenv("DISK_PATH", disk)
	cfg, err := loadMonitorConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.cpuLimit != 60.5 || cfg.ramLimit != 75 || cfg.diskLimit != 85 || cfg.interval != time.Minute || cfg.diskPath != disk {
		t.Fatalf("configuração não aplicada: %+v", cfg)
	}
}

func TestInvalidMonitorConfig(t *testing.T) {
	for _, key := range []string{"CPU_LIMIT", "RAM_LIMIT", "DISK_LIMIT"} {
		for _, value := range []string{"-1", "101", "NaN", "+Inf", "abc", "75,5"} {
			t.Run(key+"="+value, func(t *testing.T) {
				clearMonitorEnv(t)
				t.Setenv(key, value)
				if _, err := loadMonitorConfig(); err == nil || !strings.Contains(err.Error(), key) {
					t.Fatalf("configuração inválida não rejeitada: %v", err)
				}
			})
		}
	}
	for _, value := range []string{"0s", "-1s", "30", "invalido", "999999999999999999h"} {
		t.Run("interval="+value, func(t *testing.T) {
			clearMonitorEnv(t)
			t.Setenv("MONITOR_INTERVAL", value)
			if _, err := loadMonitorConfig(); err == nil {
				t.Fatal("intervalo inválido aceito")
			}
		})
	}
	t.Run("relative disk", func(t *testing.T) {
		clearMonitorEnv(t)
		t.Setenv("DISK_PATH", filepath.Join("pasta", "disco"))
		if _, err := loadMonitorConfig(); err == nil {
			t.Fatal("caminho relativo aceito")
		}
	})
}

func TestPercentageBoundaries(t *testing.T) {
	for _, value := range []string{"0", "100"} {
		t.Run(value, func(t *testing.T) {
			clearMonitorEnv(t)
			t.Setenv("CPU_LIMIT", value)
			if _, err := loadMonitorConfig(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConfiguredAlertLimits(t *testing.T) {
	cfg := monitorConfig{cpuLimit: 60.5, ramLimit: 75, diskLimit: 85, diskPath: "D:\\"}
	states := make(map[string]bool)
	var messages []string
	send := func(_ context.Context, message string) error { messages = append(messages, message); return nil }
	normal := metrics{cpu: reading{value: 60.5}, ram: reading{value: 75}, disk: reading{value: 85}}
	high := metrics{cpu: reading{value: 61}, ram: reading{value: 76}, disk: reading{value: 86}}
	checkAlerts(context.Background(), cfg, "PC", normal, states, send)
	if len(messages) != 0 {
		t.Fatal("alertou no limite exato")
	}
	checkAlerts(context.Background(), cfg, "PC", high, states, send)
	checkAlerts(context.Background(), cfg, "PC", high, states, send)
	if len(messages) != 3 {
		t.Fatalf("esperado um alerta por recurso: %v", messages)
	}
	if !strings.Contains(messages[0], "Limite: 60.5%") || !strings.Contains(messages[2], cfg.diskPath) {
		t.Fatalf("limite ou disco incorreto na mensagem: %v", messages)
	}
	checkAlerts(context.Background(), cfg, "PC", normal, states, send)
	if len(messages) != 6 {
		t.Fatalf("recuperações incorretas: %v", messages)
	}
}

func TestMonitorConfigFromEnvFile(t *testing.T) {
	clearMonitorEnv(t)
	for _, key := range []string{"CPU_LIMIT", "RAM_LIMIT", "DISK_LIMIT", "MONITOR_INTERVAL", "DISK_PATH"} {
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	disk := t.TempDir() + string(os.PathSeparator)
	path := filepath.Join(t.TempDir(), ".env")
	content := "CPU_LIMIT=65.5\nRAM_LIMIT=70\nDISK_LIMIT=80\nMONITOR_INTERVAL=45s\nDISK_PATH=" + disk + "\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadMonitorConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.cpuLimit != 65.5 || cfg.ramLimit != 70 || cfg.diskLimit != 80 || cfg.interval != 45*time.Second || cfg.diskPath != disk {
		t.Fatalf("configuração do arquivo incorreta: %+v", cfg)
	}
}
