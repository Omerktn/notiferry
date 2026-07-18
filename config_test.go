package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	c := Config{Listen: ":8080", Targets: map[string]Target{"ops": {ChatID: "-1"}}, DefaultTarget: "ops"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.DefaultTarget = "missing"
	if err := c.Validate(); err == nil {
		t.Fatal("expected missing default error")
	}
	c.DefaultTarget = "ops"
	c.TelegramBotToken = " secret"
	if err := c.Validate(); err == nil {
		t.Fatal("expected bot token whitespace error")
	}
}

func TestLoadConfigDefaultsAndStrictFields(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "config.yaml")
	if err := os.WriteFile(p, []byte("telegram_bot_token: secret\ntargets:\n  ops:\n    chat_id: '-1'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig(p)
	if err != nil || c.Listen != ":8080" || c.TelegramBotToken != "secret" {
		t.Fatalf("config=%+v err=%v", c, err)
	}
	if err := os.WriteFile(p, []byte("targets:\n  ops:\n    chat_id: '-1'\nunknown: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected strict config failure")
	}
}

func TestTelegramBotTokenEnvironmentOverride(t *testing.T) {
	c := Config{TelegramBotToken: "yaml-token"}
	t.Setenv("NOTIFERRY_TELEGRAM_BOT_TOKEN", "")
	if token := telegramBotToken(c); token != "yaml-token" {
		t.Fatalf("token=%q", token)
	}
	t.Setenv("NOTIFERRY_TELEGRAM_BOT_TOKEN", "environment-token")
	if token := telegramBotToken(c); token != "environment-token" {
		t.Fatalf("token=%q", token)
	}
}

func TestLoadConfigRejectsMultipleDocumentsAndChatIDWhitespace(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "config.yaml")
	contents := "targets:\n  ops:\n    chat_id: '-1'\n---\ntargets:\n  other:\n    chat_id: '-2'\n"
	if err := os.WriteFile(p, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected multiple document failure")
	}
	if err := os.WriteFile(p, []byte("targets:\n  ops:\n    chat_id: ' -1'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected chat ID whitespace failure")
	}
}
