package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type telegramClient struct {
	token    string
	chatID   string
	threadID int64
	baseURL  string
	client   *http.Client
	retryAt  time.Time
}

func newTelegramClient(token, chatID string, threadID int64) *telegramClient {
	return &telegramClient{
		token:    token,
		chatID:   chatID,
		threadID: threadID,
		baseURL:  "https://api.telegram.org",
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (t *telegramClient) send(ctx context.Context, message string) error {
	if time.Now().Before(t.retryAt) {
		return fmt.Errorf("Telegram solicitou aguardar antes de tentar novamente")
	}
	form := url.Values{"chat_id": {t.chatID}, "text": {message}}
	if t.threadID > 0 {
		form.Set("message_thread_id", strconv.FormatInt(t.threadID, 10))
	}
	endpoint := t.baseURL + "/bot" + t.token + "/sendMessage"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return t.safeError("criar requisição", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := t.client.Do(request)
	if err != nil {
		return t.safeError("enviar requisição", err)
	}
	defer response.Body.Close()
	const maxBody = 64 * 1024
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil {
		return t.safeError("ler resposta", err)
	}
	if len(body) > maxBody {
		return fmt.Errorf("resposta do Telegram excedeu o limite de tamanho")
	}
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("Telegram retornou resposta inválida (HTTP %d)", response.StatusCode)
	}
	if result.Parameters.RetryAfter > 0 {
		t.retryAt = time.Now().Add(time.Duration(result.Parameters.RetryAfter) * time.Second)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !result.OK {
		description := strings.ReplaceAll(result.Description, t.token, "[TOKEN REMOVIDO]")
		return fmt.Errorf("Telegram recusou mensagem (HTTP %d): %s", response.StatusCode, description)
	}
	return nil
}

// Erros de net/http podem incluir a URL, que contém o token.
func (t *telegramClient) safeError(action string, err error) error {
	return fmt.Errorf("%s: %s", action, strings.ReplaceAll(err.Error(), t.token, "[TOKEN REMOVIDO]"))
}
