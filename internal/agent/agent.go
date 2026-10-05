package agent

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type runOptions struct {
	once   bool
	dryRun bool
}

// Main inicia o agente e processa as opções de linha de comando.
func Main() {
	envPath := flag.String("env", defaultEnvPath(), "Caminho do arquivo .env")
	var options runOptions
	flag.BoolVar(&options.once, "once", false, "Executa uma coleta e encerra")
	flag.BoolVar(&options.dryRun, "dry-run", false, "Simula alertas sem enviar ao Telegram")
	flag.Parse()
	var explicitEnv bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "env" {
			explicitEnv = true
		}
	})
	if explicitEnv {
		if _, err := os.Stat(*envPath); err != nil {
			log.Printf("Erro ao acessar arquivo de configuração: %v", err)
			os.Exit(1)
		}
	}
	if err := loadEnvFile(*envPath); err != nil {
		log.Printf("Erro: %v", err)
		os.Exit(1)
	}
	if err := run(options); err != nil {
		log.Printf("Erro: %v", err)
		os.Exit(1)
	}
}

func run(options runOptions) error {
	cfg, err := loadMonitorConfig()
	if err != nil {
		return err
	}
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	chatID := strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID"))
	if !options.dryRun && (token == "" || chatID == "") {
		return fmt.Errorf("configure TELEGRAM_BOT_TOKEN e TELEGRAM_CHAT_ID")
	}
	if !options.dryRun && strings.ContainsAny(token, "/?# \t\r\n") {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN contém caracteres inválidos")
	}
	var threadID int64
	if !options.dryRun {
		threadID, err = telegramTopicID()
		if err != nil {
			return err
		}
	}
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("obter hostname: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	send := func(_ context.Context, message string) error {
		fmt.Printf("\n[SIMULAÇÃO — sem envio]\n%s\n", message)
		return nil
	}
	if !options.dryRun {
		telegram := newTelegramClient(token, chatID, threadID)
		defer telegram.client.CloseIdleConnections()
		send = telegram.send
	}
	states := make(map[string]bool)
	fmt.Println("Monitor iniciado...")
	fmt.Printf("Limites: CPU %g%%, RAM %g%%, disco %g%%; disco %s; intervalo %s\n", cfg.cpuLimit, cfg.ramLimit, cfg.diskLimit, cfg.diskPath, cfg.interval)
	if options.dryRun {
		fmt.Println("Modo simulação: nenhuma mensagem será enviada ao Telegram.")
	}
	if threadID != 0 {
		fmt.Printf("Tópico: %s (ID %d)\n", strings.TrimSpace(os.Getenv("TELEGRAM_TOPIC")), threadID)
	}
	for {
		sample := collectMetrics(ctx, cfg.diskPath)
		if ctx.Err() != nil {
			fmt.Println("\nMonitor encerrado.")
			return nil
		}
		printMetrics(hostname, sample, cfg.diskPath)
		alertErr := checkAlerts(ctx, cfg, hostname, sample, states, send)
		if ctx.Err() != nil {
			fmt.Println("\nMonitor encerrado.")
			return nil
		}
		if options.once {
			return alertErr
		}
		fmt.Printf("\nPróxima verificação em %s...\n", cfg.interval)
		// Um timer permite interromper a espera imediatamente com Ctrl+C.
		timer := time.NewTimer(cfg.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			fmt.Println("\nMonitor encerrado.")
			return nil
		case <-timer.C:
		}
	}
}

func defaultEnvPath() string {
	executable, err := os.Executable()
	if err != nil {
		return ".env"
	}
	return envPathForExecutable(executable)
}

func envPathForExecutable(executable string) string {
	path := filepath.Join(filepath.Dir(executable), ".env")
	info, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return path // Permite que o carregamento informe erros de acesso.
	}
	if err == nil && !info.IsDir() {
		return path
	}
	return ".env"
}

// Variáveis da sessão têm prioridade sobre os valores do arquivo.
func loadEnvFile(path string) error {
	if err := godotenv.Load(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Erros de parsing podem conter a linha original, incluindo credenciais.
		return fmt.Errorf("não foi possível carregar %s; verifique acesso e sintaxe", path)
	}
	return nil
}

// O nome seleciona o ID do tópico configurado para esta instalação.
func telegramTopicID() (int64, error) {
	topic := strings.ToLower(strings.TrimSpace(os.Getenv("TELEGRAM_TOPIC")))
	if topic == "" {
		return 0, nil // Mantém o envio direto ao chat para instalações sem tópicos.
	}
	var variable string
	switch topic {
	case "taiba", "charme", "magna", "acaraizinho", "wind":
		variable = "TELEGRAM_TOPIC_" + strings.ToUpper(topic) + "_ID"
	default:
		return 0, fmt.Errorf("TELEGRAM_TOPIC deve ser Taiba, charme, Magna, acaraizinho ou wind")
	}
	id, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(variable)), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("configure %s com o ID inteiro positivo do tópico", variable)
	}
	return id, nil
}
