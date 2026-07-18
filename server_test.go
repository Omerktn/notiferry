package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSender struct {
	calls  []string
	failAt int
}

func (f *fakeSender) Send(_ context.Context, _ Target, text, _ string) error {
	f.calls = append(f.calls, text)
	if f.failAt > 0 && len(f.calls) == f.failAt {
		return context.DeadlineExceeded
	}
	return nil
}

func testApp(sender Sender) *App {
	return NewApp(Config{Targets: map[string]Target{"ops": {ChatID: "1"}}, DefaultTarget: "ops"}, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
func TestNotifyValidationAndReceipt(t *testing.T) {
	f := &fakeSender{}
	ts := httptest.NewServer(testApp(f).Handler())
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/v1/notify", "application/json", strings.NewReader(`{"text":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || len(f.calls) != 1 {
		t.Fatalf("status=%d calls=%d", resp.StatusCode, len(f.calls))
	}
	var r receipt
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil || r.Target != "ops" || r.Chunks != 1 {
		t.Fatalf("receipt=%+v err=%v", r, err)
	}
	resp, _ = http.Post(ts.URL+"/v1/notify", "application/json", strings.NewReader(`{"text":"x","format":"markdown"}`))
	if resp.StatusCode != 400 {
		t.Fatalf("format status=%d", resp.StatusCode)
	}
	resp.Body.Close()
	resp, err = http.Post(ts.URL+"/v1/notify", "application/json", strings.NewReader(`{"text":"x"} {}`))
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("trailing JSON status=%v err=%v", resp.Status, err)
	}
	var trailing apiError
	_ = json.NewDecoder(resp.Body).Decode(&trailing)
	resp.Body.Close()
	if trailing.RequestID == "" {
		t.Fatal("validation error has no request ID")
	}
}

func TestHTMLLimit(t *testing.T) {
	ts := httptest.NewServer(testApp(&fakeSender{}).Handler())
	defer ts.Close()
	text := strings.Repeat("界", telegramLimit+1)
	resp, err := http.Post(ts.URL+"/v1/notify", "application/json", strings.NewReader(`{"text":"`+text+`","format":"html"}`))
	if err != nil || resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%v err=%v", resp.Status, err)
	}
	resp.Body.Close()
}

func TestTargetRequiredAndOversizedBody(t *testing.T) {
	a := NewApp(Config{Targets: map[string]Target{"ops": {ChatID: "1"}}}, &fakeSender{}, nil)
	ts := httptest.NewServer(a.Handler())
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/v1/notify", "application/json", strings.NewReader(`{"text":"hello"}`))
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("target required status=%v err=%v", resp.Status, err)
	}
	resp.Body.Close()
	body := `{"text":"` + strings.Repeat("x", 130*1024) + `"}`
	resp, err = http.Post(ts.URL+"/v1/notify", "application/json", strings.NewReader(body))
	if err != nil || resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status=%v err=%v", resp.Status, err)
	}
	resp.Body.Close()

	resp, err = http.Post(ts.URL+"/v1/notify", "application/json", strings.NewReader(`{"target":"missing","text":"hello"}`))
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown target status=%v err=%v", resp.Status, err)
	}
	resp.Body.Close()
}
func TestPartialDelivery(t *testing.T) {
	f := &fakeSender{failAt: 2}
	ts := httptest.NewServer(testApp(f).Handler())
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/v1/notify", "application/json", strings.NewReader(`{"text":"`+repeat("a", 5000)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Fatal("expected delivery failure")
	}
	var e apiError
	_ = json.NewDecoder(resp.Body).Decode(&e)
	if e.Receipt == nil || e.Receipt.Delivered != 1 {
		t.Fatalf("error=%+v", e)
	}
}

func TestHealthAndReload(t *testing.T) {
	f := &fakeSender{}
	a := testApp(f)
	ts := httptest.NewServer(a.Handler())
	defer ts.Close()
	for _, path := range []string{"/health/live", "/health/ready"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("health %s: %v", path, err)
		}
		resp.Body.Close()
	}
	a.Reload(Config{Listen: ":9090", Targets: map[string]Target{"new": {ChatID: "2"}}, DefaultTarget: "new"})
	resp, err := http.Post(ts.URL+"/v1/notify", "application/json", strings.NewReader(`{"text":"x"}`))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("reload status=%v err=%v", resp.StatusCode, err)
	}
	resp.Body.Close()
}
func repeat(s string, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = s[0]
	}
	return string(b)
}
