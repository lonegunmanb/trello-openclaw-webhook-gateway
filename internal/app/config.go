package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

type Config struct {
	ListenAddr   string
	TrelloSecret string
	CallbackURL  string
	ForwardURL   string
	ForwardToken string
	Model        string
}

func LoadConfig(args []string) (Config, error) {
	cfg := Config{
		ListenAddr:   envOrDefault("LISTEN_ADDR", ":18790"),
		TrelloSecret: os.Getenv("TRELLO_API_SECRET"),
		CallbackURL:  os.Getenv("CALLBACK_URL"),
		ForwardURL:   os.Getenv("FORWARD_URL"),
		ForwardToken: os.Getenv("FORWARD_TOKEN"),
		Model:        os.Getenv("MODEL"),
	}

	fs := flag.NewFlagSet("gateway", flag.ContinueOnError)
	fs.StringVar(&cfg.ListenAddr, "listen", cfg.ListenAddr, "listen address")
	fs.StringVar(&cfg.TrelloSecret, "trello-api-secret", cfg.TrelloSecret, "Trello API secret")
	fs.StringVar(&cfg.CallbackURL, "callback-url", cfg.CallbackURL, "webhook callback URL used for signature verification")
	fs.StringVar(&cfg.ForwardURL, "forward-url", cfg.ForwardURL, "OpenClaw webhook URL")
	fs.StringVar(&cfg.ForwardToken, "forward-token", cfg.ForwardToken, "OpenClaw bearer token")
	fs.StringVar(&cfg.Model, "model", cfg.Model, "model for OpenClaw webhook processing")

	if err := fs.Parse(args[1:]); err != nil {
		return Config{}, err
	}

	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validateConfig(cfg Config) error {
	if cfg.TrelloSecret == "" {
		return errors.New("missing trello secret, set --trello-api-secret or TRELLO_API_SECRET")
	}
	if cfg.CallbackURL == "" {
		return errors.New("missing callback URL, set --callback-url or CALLBACK_URL")
	}
	if cfg.ForwardURL == "" {
		return errors.New("missing forward URL, set --forward-url or FORWARD_URL")
	}
	if cfg.ForwardToken == "" {
		return errors.New("missing forward token, set --forward-token or FORWARD_TOKEN")
	}
	if cfg.Model == "" {
		return errors.New("missing model, set --model or MODEL")
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func (c Config) Redacted() string {
	return fmt.Sprintf("listen=%s callback_url=%s forward_url=%s trello_api_secret=%s forward_token=%s model=%s", c.ListenAddr, c.CallbackURL, c.ForwardURL, redact(c.TrelloSecret), redact(c.ForwardToken), c.Model)
}

func redact(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 4 {
		return "****"
	}
	return v[:2] + "***" + v[len(v)-2:]
}
