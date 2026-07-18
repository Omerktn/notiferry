package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen        string            `yaml:"listen"`
	DefaultTarget string            `yaml:"default_target"`
	Targets       map[string]Target `yaml:"targets"`
}

type Target struct {
	ChatID  string `yaml:"chat_id"`
	TopicID int64  `yaml:"topic_id,omitempty"`
}

func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, errors.New("parse config: multiple YAML documents are not supported")
		}
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Listen) == "" {
		return errors.New("listen must not be empty")
	}
	if len(c.Targets) == 0 {
		return errors.New("targets must contain at least one target")
	}
	for alias, target := range c.Targets {
		if strings.TrimSpace(alias) == "" || strings.TrimSpace(alias) != alias {
			return fmt.Errorf("invalid target alias %q", alias)
		}
		if strings.TrimSpace(target.ChatID) == "" {
			return fmt.Errorf("target %q chat_id is required", alias)
		}
		if strings.TrimSpace(target.ChatID) != target.ChatID {
			return fmt.Errorf("target %q chat_id must not have surrounding whitespace", alias)
		}
		if target.TopicID < 0 {
			return fmt.Errorf("target %q topic_id must be positive", alias)
		}
	}
	if c.DefaultTarget != "" {
		if _, ok := c.Targets[c.DefaultTarget]; !ok {
			return fmt.Errorf("default_target %q is not configured", c.DefaultTarget)
		}
	}
	return nil
}

func (c Config) Resolve(alias string) (string, Target, error) {
	if alias == "" {
		alias = c.DefaultTarget
	}
	t, ok := c.Targets[alias]
	if !ok {
		return "", Target{}, fmt.Errorf("unknown target %q", alias)
	}
	return alias, t, nil
}

func (t Target) chatID() string { return t.ChatID }
