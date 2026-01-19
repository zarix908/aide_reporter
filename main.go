package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"text/template"

	"log"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/mymmrac/telego"

	_ "embed"
)

//go:embed message.templ
var messageTemplate []byte

type Config struct {
	Token  string     `yaml:"token"`
	ChatId int64      `yaml:"chat_id"`
	Aide   AideConfig `yaml:"aide"`
}

type AideConfig struct {
	ConfigPath  string `yaml:"config_path"`
	DatabaseIn  string `yaml:"database_in"`
	DatabaseOut string `yaml:"database_out"`
}

type Report struct {
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	AideVersion   string `json:"aide_version"`
	EntriesNumber struct {
		Total   int `json:"total"`
		Added   int `json:"added"`
		Changed int `json:"changed"`
		Removed int `json:"removed"`
	} `json:"number_of_entries"`

	Added   map[string]any `json:"added"`
	Changed map[string]any `json:"changed"`
	Removed map[string]any `json:"removed"`

	Details map[string]struct {
		Sha256 struct {
			Old string `json:"old"`
			New string `json:"new"`
		} `json:"sha256"`
	} `json:"details"`

	Databases map[string]struct {
		Sha256 string `json:"sha256"`
	} `json:"databases"`
}

func main() {
	if err := sendReport(); err != nil {
		log.Printf("error occured: %v\n", err)
	}
}

func sendReport() error {
	config, err := os.ReadFile("./config/config.yaml")
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cmd := exec.CommandContext(context.Background(), "aide", "--config", cfg.Aide.ConfigPath, "--check")
	aideOutput, err := cmd.Output()
	if err != nil {
		log.Printf("aide command exited with error %v", err)
	}

	var r Report
	if err := json.Unmarshal([]byte(aideOutput), &r); err != nil {
		return fmt.Errorf("failed to unmarshal aide report: %w", err)
	}

	msgTempl, err := template.New("message.templ").Parse(string(messageTemplate))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	msg := bytes.NewBuffer(nil)
	if err := msgTempl.Execute(msg, &r); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	bot, err := telego.NewBot(cfg.Token)
	if err != nil {
		return fmt.Errorf("failed to create bot: %w", err)
	}

	if _, err = bot.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:    telego.ChatID{ID: cfg.ChatId},
		Text:      msg.String(),
		ParseMode: "MarkdownV2",
	}); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	cmd = exec.CommandContext(context.Background(), "aide", "--config", cfg.Aide.ConfigPath, "--update")
	aideOutput, err = cmd.Output()
	if err != nil {
		log.Printf("aide command exited with error %v", err)
	}
	fmt.Printf("aide output after update: %s", string(aideOutput))

	if err := os.Rename(cfg.Aide.DatabaseOut, cfg.Aide.DatabaseIn); err != nil {
		return fmt.Errorf("failed to replace database: %w", err)
	}

	return nil
}
