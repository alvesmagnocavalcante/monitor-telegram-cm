package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	for _, name := range []string{"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("TELEGRAM_BOT_TOKEN='123:fake'\nTELEGRAM_CHAT_ID=-100123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("TELEGRAM_BOT_TOKEN") != "123:fake" || os.Getenv("TELEGRAM_CHAT_ID") != "-100123" {
		t.Fatal("credenciais não foram carregadas")
	}
}

func TestLoadEnvPreservesExistingVariables(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "123:session")
	t.Setenv("TELEGRAM_CHAT_ID", "456")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("TELEGRAM_BOT_TOKEN=123:file\nTELEGRAM_CHAT_ID=789\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("TELEGRAM_BOT_TOKEN") != "123:session" || os.Getenv("TELEGRAM_CHAT_ID") != "456" {
		t.Fatal("arquivo sobrescreveu variáveis existentes")
	}
}

func TestLoadMissingEnvFile(t *testing.T) {
	if err := loadEnvFile(filepath.Join(t.TempDir(), ".env")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadInvalidEnvDoesNotExposeSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	secret := "123:private-secret"
	if err := os.WriteFile(path, []byte("TELEGRAM_BOT_TOKEN=\""+secret+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err := loadEnvFile(path)
	if err == nil {
		t.Fatal("arquivo inválido foi aceito")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("credencial exposta no erro")
	}
}

func TestTelegramTopicID(t *testing.T) {
	for _, name := range []string{"TAIBA", "CHARME", "MAGNA", "ACARAIZINHO", "WIND"} {
		t.Setenv("TELEGRAM_TOPIC_"+name+"_ID", "")
	}
	cases := []struct {
		name      string
		topic     string
		id        string
		want      int64
		wantError bool
	}{
		{"compatibilidade", "", "", 0, false},
		{"taiba", "Taiba", "11", 11, false},
		{"charme", "charme", "22", 22, false},
		{"magna", "Magna", "33", 33, false},
		{"acaraizinho", "acaraizinho", "44", 44, false},
		{"wind", "wind", "55", 55, false},
		{"espaços", " TAIBA ", " 11 ", 11, false},
		{"desconhecido", "outro", "11", 0, true},
		{"ausente", "Taiba", "", 0, true},
		{"zero", "Taiba", "0", 0, true},
		{"negativo", "Taiba", "-1", 0, true},
		{"texto", "Taiba", "abc", 0, true},
		{"decimal", "Taiba", "1.5", 0, true},
		{"overflow", "Taiba", "9223372036854775808", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TELEGRAM_TOPIC", tc.topic)
			if key := strings.ToUpper(strings.TrimSpace(tc.topic)); key != "" {
				t.Setenv("TELEGRAM_TOPIC_"+key+"_ID", tc.id)
			}
			got, err := telegramTopicID()
			if (err != nil) != tc.wantError || got != tc.want {
				t.Fatalf("resultado: %d, erro: %v", got, err)
			}
		})
	}
}

func TestEnvPathForExecutable(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "monitor-telegram.exe")
	if got := envPathForExecutable(executable); got != ".env" {
		t.Fatalf("esperado fallback ao diretório atual, recebido %s", got)
	}
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("RAM_LIMIT=75\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := envPathForExecutable(executable); got != path {
		t.Fatalf("arquivo ao lado do executável não foi escolhido: %s", got)
	}
}

func TestEnvDirectoryIsNotSelected(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".env"), 0700); err != nil {
		t.Fatal(err)
	}
	if got := envPathForExecutable(filepath.Join(dir, "monitor.exe")); got != ".env" {
		t.Fatalf("selecionou um diretório como arquivo: %s", got)
	}
}
