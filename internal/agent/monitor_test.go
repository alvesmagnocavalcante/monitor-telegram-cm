package agent

import (
	"context"
	"errors"
	"math"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAlertTransitions(t *testing.T) {
	for _, metric := range []string{"CPU", "RAM", "DISCO"} {
		t.Run(metric, func(t *testing.T) {
			states := make(map[string]bool)
			var messages []string
			send := func(_ context.Context, message string) error {
				messages = append(messages, message)
				return nil
			}
			sample := func(value float64, err error) metrics {
				var m metrics
				r := reading{value: value, err: err}
				switch metric {
				case "CPU":
					m.cpu = r
				case "RAM":
					m.ram = r
				case "DISCO":
					m.disk = r
				}
				return m
			}
			for _, value := range []float64{90, 91, 95} {
				checkAlerts(context.Background(), testMonitorConfig(), "PC", sample(value, nil), states, send)
			}
			if len(messages) != 1 || !states[metric] {
				t.Fatalf("esperado um alerta e estado ativo: %v, %v", messages, states)
			}
			checkAlerts(context.Background(), testMonitorConfig(), "PC", sample(0, errors.New("coleta falhou")), states, send)
			if len(messages) != 1 || !states[metric] {
				t.Fatal("falha de coleta alterou o estado")
			}
			checkAlerts(context.Background(), testMonitorConfig(), "PC", sample(90, nil), states, send)
			checkAlerts(context.Background(), testMonitorConfig(), "PC", sample(40, nil), states, send)
			checkAlerts(context.Background(), testMonitorConfig(), "PC", sample(99, nil), states, send)
			if len(messages) != 3 || !strings.Contains(messages[1], "NORMALIZADO") || !states[metric] {
				t.Fatalf("transições incorretas: %v, %v", messages, states)
			}
		})
	}
}

func TestFailedNotificationRetried(t *testing.T) {
	states := make(map[string]bool)
	sample := metrics{cpu: reading{value: 95}}
	failure := func(context.Context, string) error { return errors.New("offline") }
	checkAlerts(context.Background(), testMonitorConfig(), "PC", sample, states, failure)
	if states["CPU"] {
		t.Fatal("envio falho confirmou alerta")
	}
	success := func(context.Context, string) error { return nil }
	checkAlerts(context.Background(), testMonitorConfig(), "PC", sample, states, success)
	if !states["CPU"] {
		t.Fatal("nova tentativa não confirmou alerta")
	}
	sample.cpu.value = 50
	checkAlerts(context.Background(), testMonitorConfig(), "PC", sample, states, failure)
	if !states["CPU"] {
		t.Fatal("envio falho confirmou recuperação")
	}
	checkAlerts(context.Background(), testMonitorConfig(), "PC", sample, states, success)
	if states["CPU"] {
		t.Fatal("recuperação não foi confirmada")
	}
}

func TestCanceledAlerts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checkAlerts(ctx, testMonitorConfig(), "PC", metrics{cpu: reading{value: 99}}, make(map[string]bool),
		func(context.Context, string) error { t.Fatal("enviou após cancelamento"); return nil })
}

func TestFormatUptime(t *testing.T) {
	if got := formatUptime(3*86400 + 4*3600 + 21*60 + 59); got != "3d 4h 21m" {
		t.Fatalf("uptime incorreto: %s", got)
	}
}

func TestNotificationsIncludeUptime(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		for _, resource := range []string{"CPU", "RAM", "DISCO"} {
			sample := metrics{uptime: 3*86400 + 4*3600 + 21*60}
			want := "Uptime: 3d 4h 21m"
			if unavailable {
				sample.uptimeErr = errors.New("falha ao ler uptime")
				want = "Uptime: indisponível"
			}
			var messages []string
			states := make(map[string]bool)
			for _, value := range []float64{99, 40} {
				reading := reading{value: value}
				switch resource {
				case "CPU":
					sample.cpu = reading
				case "RAM":
					sample.ram = reading
				case "DISCO":
					sample.disk = reading
				}
				if err := checkAlerts(context.Background(), testMonitorConfig(), "PC", sample, states,
					func(_ context.Context, message string) error {
						messages = append(messages, message)
						return nil
					}); err != nil {
					t.Fatal(err)
				}
			}
			if len(messages) != 2 {
				t.Fatalf("%s: esperado alerta e recuperação, recebido %v", resource, messages)
			}
			for _, message := range messages {
				if !strings.Contains(message, want) {
					t.Fatalf("%s: uptime incorreto na notificação: %s", resource, message)
				}
			}
		}
	}
}

func TestMissingCredentials(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	if err := run(runOptions{}); err == nil {
		t.Fatal("aceitou credenciais ausentes")
	}
}

func TestDryRunOnceWithoutTelegramCredentials(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("coleta do disco C: requer Windows")
	}
	clearMonitorEnv(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("TELEGRAM_TOPIC", "sem configuração de destino")
	if err := run(runOptions{once: true, dryRun: true}); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidReadingsPreserveAlertState(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, 101} {
		for _, active := range []bool{false, true} {
			states := map[string]bool{"CPU": active}
			checkAlerts(context.Background(), testMonitorConfig(), "PC", metrics{cpu: reading{value: value}}, states,
				func(context.Context, string) error { t.Fatal("notificação para leitura inválida"); return nil })
			if states["CPU"] != active {
				t.Fatal("leitura inválida alterou o estado")
			}
		}
	}
}

func TestAlertSendErrorIsReturned(t *testing.T) {
	want := errors.New("falha de conexão")
	states := make(map[string]bool)
	err := checkAlerts(context.Background(), testMonitorConfig(), "PC", metrics{cpu: reading{value: 99}}, states,
		func(context.Context, string) error { return want })
	if !errors.Is(err, want) || states["CPU"] {
		t.Fatalf("envio falho confirmado ou erro perdido: %v", err)
	}
}

func TestCollectMetricsWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("coleta do disco C: requer Windows")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sample := collectMetrics(ctx, testMonitorConfig().diskPath)
	for name, r := range map[string]reading{"CPU": sample.cpu, "RAM": sample.ram, "DISCO": sample.disk} {
		if r.err != nil {
			t.Fatalf("%s: %v", name, r.err)
		}
		if r.value < 0 || r.value > 100 {
			t.Fatalf("%s inválido: %f", name, r.value)
		}
	}
	if sample.uptimeErr != nil {
		t.Fatal(sample.uptimeErr)
	}
	if sample.uptime == 0 {
		t.Fatal("uptime zerado")
	}
	t.Logf("CPU %.1f%%, RAM %.1f%%, disco %.1f%%, uptime %s", sample.cpu.value, sample.ram.value, sample.disk.value, formatUptime(sample.uptime))
}

func testMonitorConfig() monitorConfig {
	return monitorConfig{cpuLimit: 90, ramLimit: 90, diskLimit: 90, interval: 30 * time.Second, diskPath: "C:\\"}
}
