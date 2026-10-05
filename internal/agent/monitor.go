package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

type reading struct {
	value float64
	err   error
}

type metrics struct {
	cpu       reading
	ram       reading
	disk      reading
	uptime    uint64
	uptimeErr error
}

func collectMetrics(ctx context.Context, diskPath string) metrics {
	var result metrics
	percentages, err := cpu.PercentWithContext(ctx, time.Second, false)
	result.cpu.err = err
	if err == nil {
		if len(percentages) == 0 {
			result.cpu.err = fmt.Errorf("nenhuma amostra de CPU retornada")
		} else {
			result.cpu.value = percentages[0]
		}
	}
	memory, err := mem.VirtualMemoryWithContext(ctx)
	result.ram.err = err
	if err == nil {
		result.ram.value = memory.UsedPercent
	}
	usage, err := disk.UsageWithContext(ctx, diskPath)
	result.disk.err = err
	if err == nil {
		result.disk.value = usage.UsedPercent
	}
	result.uptime, result.uptimeErr = host.UptimeWithContext(ctx)
	return result
}

func printMetrics(hostname string, sample metrics, diskPath string) {
	fmt.Printf("\nHost: %s\n", hostname)
	printReading("CPU", sample.cpu)
	printReading("RAM", sample.ram)
	printReading(fmt.Sprintf("Disco (%s)", diskPath), sample.disk)
	if sample.uptimeErr != nil {
		log.Printf("Erro ao ler uptime: %v", sample.uptimeErr)
	} else {
		fmt.Printf("Uptime: %s\n", formatUptime(sample.uptime))
	}
}

func printReading(label string, value reading) {
	if value.err != nil {
		log.Printf("Erro ao ler %s: %v", label, value.err)
		return
	}
	fmt.Printf("%s: %.1f%%\n", label, value.value)
}

func formatUptime(seconds uint64) string {
	return fmt.Sprintf("%dd %dh %dm", seconds/86400, seconds%86400/3600, seconds%3600/60)
}

func checkAlerts(ctx context.Context, cfg monitorConfig, hostname string, sample metrics, states map[string]bool, send func(context.Context, string) error) error {
	var sendErrors error
	uptime := "indisponível"
	if sample.uptimeErr == nil {
		uptime = formatUptime(sample.uptime)
	}
	checks := []struct {
		name    string
		label   string
		limit   float64
		reading reading
	}{
		{"CPU", "CPU", cfg.cpuLimit, sample.cpu},
		{"RAM", "RAM", cfg.ramLimit, sample.ram},
		{"DISCO", fmt.Sprintf("Disco (%s)", cfg.diskPath), cfg.diskLimit, sample.disk},
	}
	for _, check := range checks {
		if ctx.Err() != nil {
			return errors.Join(sendErrors, ctx.Err())
		}
		if check.reading.err != nil {
			continue // Uma falha de coleta não é uma recuperação.
		}
		if math.IsNaN(check.reading.value) || math.IsInf(check.reading.value, 0) || check.reading.value < 0 || check.reading.value > 100 {
			log.Printf("Leitura inválida de %s; estado do alerta preservado", check.name)
			continue
		}
		above := check.reading.value > check.limit
		if above == states[check.name] {
			continue
		}
		var message string
		if above {
			message = fmt.Sprintf("⚠️ ALERTA DE %s\n\nHost: %s\n%s: %.1f%%\nLimite: %g%%", check.name, hostname, check.label, check.reading.value, check.limit)
		} else {
			message = fmt.Sprintf("🟢 %s NORMALIZADO(A)\n\nHost: %s\n%s atual: %.1f%%", check.name, hostname, check.label, check.reading.value)
		}
		message += "\nUptime: " + uptime
		if err := send(ctx, message); err != nil {
			sendErrors = errors.Join(sendErrors, fmt.Errorf("notificação de %s: %w", check.name, err))
			if ctx.Err() == nil {
				log.Printf("Erro ao enviar notificação de %s: %v", check.name, err)
			}
			continue
		}
		// Confirma o estado somente após o Telegram aceitar a mensagem.
		states[check.name] = above
	}
	return sendErrors
}
