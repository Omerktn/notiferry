package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Sender interface {
	Send(ctx context.Context, target Target, text, format string) error
}

type TelegramClient struct {
	Token   string
	HTTP    *http.Client
	BaseURL string
	Sleep   func(context.Context, time.Duration) error
}

type telegramResponse struct {
	OK          bool               `json:"ok"`
	Description string             `json:"description"`
	Parameters  telegramParameters `json:"parameters"`
}

type telegramParameters struct {
	RetryAfter *int `json:"retry_after"`
}

func (c *TelegramClient) endpoint(method string) string {
	return strings.TrimRight(c.BaseURL, "/") + "/bot" + c.Token + "/" + method
}

func (c *TelegramClient) request(ctx context.Context, method string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	sleep := c.Sleep
	if sleep == nil {
		sleep = func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		}
	}
	// Bound the complete request, including retries and backoff. A caller may
	// still impose a shorter deadline through ctx.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	start := time.Now()
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(method), strings.NewReader(string(b)))
		if err != nil {
			return fmt.Errorf("telegram request creation failed")
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := hc.Do(req)
		if err != nil {
			// Do not return the standard error: it can contain the bot URL/token.
			return fmt.Errorf("telegram transport failed")
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("telegram response read failed")
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var tr telegramResponse
			if err := json.Unmarshal(body, &tr); err != nil {
				return fmt.Errorf("telegram returned invalid JSON")
			}
			if !tr.OK {
				return fmt.Errorf("telegram: %s", tr.Description)
			}
			return nil
		}
		var tr telegramResponse
		_ = json.Unmarshal(body, &tr)
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if !retry || attempt == 3 {
			return fmt.Errorf("telegram HTTP %d", resp.StatusCode)
		}
		delay := 250 * time.Millisecond
		headerDelaySet := false
		if v := resp.Header.Get("Retry-After"); v != "" {
			if n, e := strconv.Atoi(v); e == nil && n >= 0 {
				delay = time.Duration(n) * time.Second
				headerDelaySet = true
			}
		}
		if !headerDelaySet && resp.StatusCode == http.StatusTooManyRequests && tr.Parameters.RetryAfter != nil && *tr.Parameters.RetryAfter >= 0 {
			delay = time.Duration(*tr.Parameters.RetryAfter) * time.Second
		}
		remaining := 10*time.Second - time.Since(start)
		if delay > remaining || remaining <= 0 {
			return fmt.Errorf("telegram retry budget exhausted (HTTP %d)", resp.StatusCode)
		}
		if err := sleep(ctx, delay); err != nil {
			return err
		}
	}
	return fmt.Errorf("telegram request failed")
}

func (c *TelegramClient) Validate(ctx context.Context) error {
	return c.request(ctx, "getMe", map[string]any{})
}

func (c *TelegramClient) Send(ctx context.Context, target Target, text, format string) error {
	p := map[string]any{"chat_id": target.chatID(), "text": text}
	if format == "html" {
		p["parse_mode"] = "HTML"
	}
	if target.TopicID != 0 {
		p["message_thread_id"] = target.TopicID
	}
	return c.request(ctx, "sendMessage", p)
}
