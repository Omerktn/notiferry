package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTelegramGetMeAndSend(t *testing.T) {
	var methods []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true}`)
	}))
	defer ts.Close()
	c := &TelegramClient{Token: "secret", BaseURL: ts.URL}
	if err := c.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.Send(context.Background(), Target{ChatID: "-4", TopicID: 7}, "hi", "html"); err != nil {
		t.Fatal(err)
	}
	if len(methods) != 2 || !strings.HasSuffix(methods[1], "/sendMessage") {
		t.Fatalf("methods=%v", methods)
	}
}

func TestTelegramRetries429AndNotTransport(t *testing.T) {
	count := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		io.WriteString(w, `{"ok":true}`)
	}))
	defer ts.Close()
	sleeps := 0
	c := &TelegramClient{Token: "x", BaseURL: ts.URL, Sleep: func(context.Context, time.Duration) error { sleeps++; return nil }}
	if err := c.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count != 3 || sleeps != 2 {
		t.Fatalf("count=%d sleeps=%d", count, sleeps)
	}
	bad := &TelegramClient{Token: "x", BaseURL: "http://127.0.0.1:1", HTTP: &http.Client{Timeout: time.Millisecond}}
	if err := bad.Validate(context.Background()); err == nil {
		t.Fatal("expected transport error")
	}
}

func TestTelegramRetriesUsingJSONRetryAfter(t *testing.T) {
	count := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"ok":false,"parameters":{"retry_after":1}}`)
			return
		}
		io.WriteString(w, `{"ok":true}`)
	}))
	defer ts.Close()
	sleeps := []time.Duration{}
	c := &TelegramClient{Token: "x", BaseURL: ts.URL, Sleep: func(_ context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		return nil
	}}
	if err := c.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sleeps) != 1 || sleeps[0] != time.Second {
		t.Fatalf("sleeps=%v", sleeps)
	}
}

func TestTelegramPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		_ = json.NewDecoder(r.Body).Decode(&p)
		if p["parse_mode"] != "HTML" {
			t.Errorf("payload=%v", p)
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()
	if err := (&TelegramClient{Token: "x", BaseURL: ts.URL}).Send(context.Background(), Target{ChatID: "chat"}, "x", "html"); err != nil {
		t.Fatal(err)
	}
}
