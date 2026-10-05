package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestTelegramResponses(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		body      string
		wantError bool
	}{
		{"success", 200, `{"ok":true}`, false},
		{"api rejection", 200, `{"ok":false,"description":"chat not found"}`, true},
		{"unauthorized", 401, `{"ok":false,"description":"Unauthorized"}`, true},
		{"invalid json", 502, "invalid", true},
		{"oversized", 200, strings.Repeat("x", 65537), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/bot123:test/sendMessage" {
					t.Errorf("requisição incorreta: %s %s", r.Method, r.URL.Path)
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Form.Get("chat_id") != "-100123" || r.Form.Get("text") != "Olá & teste" {
					t.Errorf("formulário incorreto: %v", r.Form)
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client := newTelegramClient("123:test", "-100123", 0)
			client.baseURL = server.URL
			err := client.send(context.Background(), "Olá & teste")
			if (err != nil) != tc.wantError {
				t.Fatalf("erro inesperado: %v", err)
			}
		})
	}
}

func TestTelegramRetryAfter(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(429)
		fmt.Fprint(w, `{"ok":false,"parameters":{"retry_after":60}}`)
	}))
	defer server.Close()
	client := newTelegramClient("123:test", "123", 0)
	client.baseURL = server.URL
	for i := 0; i < 2; i++ {
		if err := client.send(context.Background(), "teste"); err == nil {
			t.Fatal("esperado erro")
		}
	}
	if calls != 1 {
		t.Fatalf("ignorou retry_after: %d chamadas", calls)
	}
}

func TestTelegramCancellationRedactsToken(t *testing.T) {
	client := newTelegramClient("123:secret", "123", 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := client.send(ctx, "teste")
	if err == nil || strings.Contains(err.Error(), client.token) {
		t.Fatalf("cancelamento ou proteção do token incorretos: %v", err)
	}
}

func TestTelegramRejectsRedirect(t *testing.T) {
	destinationCalls := 0
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationCalls++
	}))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client := newTelegramClient("123:test", "123", 0)
	client.baseURL = server.URL
	if err := client.send(context.Background(), "teste"); err == nil {
		t.Fatal("aceitou redirect")
	}
	if destinationCalls != 0 {
		t.Fatal("seguiu redirect")
	}
}

func TestTelegramTopicRouting(t *testing.T) {
	cases := []struct {
		name string
		id   int64
	}{
		{"sem tópico", 0}, {"Taiba", 11}, {"charme", 22},
		{"Magna", 33}, {"acaraizinho", 44}, {"wind", 55},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				got := r.Form.Get("message_thread_id")
				if tc.id == 0 {
					if _, present := r.Form["message_thread_id"]; present {
						t.Error("enviou ID para chat sem tópico")
					}
				} else if got != strconv.FormatInt(tc.id, 10) {
					t.Errorf("tópico incorreto: %q", got)
				}
				if r.Form.Get("chat_id") != "-100123" {
					t.Error("chat incorreto")
				}
				fmt.Fprint(w, `{"ok":true}`)
			}))
			defer server.Close()
			client := newTelegramClient("123:test", "-100123", tc.id)
			client.baseURL = server.URL
			states := make(map[string]bool)
			for _, value := range []float64{99, 99, 40} {
				checkAlerts(context.Background(), testMonitorConfig(), "PC", metrics{cpu: reading{value: value}}, states, client.send)
			}
			if calls != 2 {
				t.Fatalf("esperado alerta e recuperação: %d envios", calls)
			}
		})
	}
}
