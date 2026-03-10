package app

import "testing"

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("LISTEN_ADDR", ":9090")
	t.Setenv("TRELLO_API_SECRET", "env-secret")
	t.Setenv("CALLBACK_URL", "https://env.example.com/trello")
	t.Setenv("FORWARD_URL", "http://127.0.0.1:18789/hooks/agent")
	t.Setenv("FORWARD_TOKEN", "env-token")
	t.Setenv("MODEL", "copilot-api/claude-haiku-4.5")
	t.Setenv("PROMPT", "Please process Trello events")

	cfg, err := LoadConfig([]string{"cmd"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.ListenAddr != ":9090" {
		t.Fatalf("unexpected listen: %s", cfg.ListenAddr)
	}
	if cfg.TrelloSecret != "env-secret" {
		t.Fatalf("unexpected secret: %s", cfg.TrelloSecret)
	}
	if cfg.CallbackURL != "https://env.example.com/trello" {
		t.Fatalf("unexpected callback: %s", cfg.CallbackURL)
	}
	if cfg.Model != "copilot-api/claude-haiku-4.5" {
		t.Fatalf("unexpected model: %s", cfg.Model)
	}
	if cfg.Prompt != "Please process Trello events" {
		t.Fatalf("unexpected prompt: %s", cfg.Prompt)
	}
}

func TestLoadConfigFlagOverridesEnv(t *testing.T) {
	t.Setenv("LISTEN_ADDR", ":9090")
	t.Setenv("TRELLO_API_SECRET", "env-secret")
	t.Setenv("CALLBACK_URL", "https://env.example.com/trello")
	t.Setenv("FORWARD_URL", "http://127.0.0.1:18789/hooks/agent")
	t.Setenv("FORWARD_TOKEN", "env-token")
	t.Setenv("MODEL", "copilot-api/claude-haiku-4.5")
	t.Setenv("PROMPT", "env prompt")

	cfg, err := LoadConfig([]string{"cmd", "--listen", ":8088", "--trello-api-secret", "flag-secret", "--model", "custom-model", "--prompt", "flag prompt"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.ListenAddr != ":8088" {
		t.Fatalf("expected listen from flag, got %s", cfg.ListenAddr)
	}
	if cfg.TrelloSecret != "flag-secret" {
		t.Fatalf("expected secret from flag, got %s", cfg.TrelloSecret)
	}
	if cfg.Model != "custom-model" {
		t.Fatalf("expected model from flag, got %s", cfg.Model)
	}
	if cfg.Prompt != "flag prompt" {
		t.Fatalf("expected prompt from flag, got %s", cfg.Prompt)
	}
}

func TestLoadConfigMissingRequired(t *testing.T) {
	_, err := LoadConfig([]string{"cmd"})
	if err == nil {
		t.Fatal("expected error when required fields are missing")
	}
}

func TestLoadConfigDefaultListenAddr(t *testing.T) {
	t.Setenv("TRELLO_API_SECRET", "env-secret")
	t.Setenv("CALLBACK_URL", "https://env.example.com/trello")
	t.Setenv("FORWARD_URL", "http://127.0.0.1:18789/hooks/agent")
	t.Setenv("FORWARD_TOKEN", "env-token")
	t.Setenv("MODEL", "copilot-api/claude-haiku-4.5")
	t.Setenv("PROMPT", "Please process Trello events")

	cfg, err := LoadConfig([]string{"cmd"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.ListenAddr != ":18790" {
		t.Fatalf("expected default listen addr :18790, got %s", cfg.ListenAddr)
	}
}

func TestLoadConfigModelRequired(t *testing.T) {
	t.Setenv("TRELLO_API_SECRET", "env-secret")
	t.Setenv("CALLBACK_URL", "https://env.example.com/trello")
	t.Setenv("FORWARD_URL", "http://127.0.0.1:18789/hooks/agent")
	t.Setenv("FORWARD_TOKEN", "env-token")
	t.Setenv("PROMPT", "Please process Trello events")

	_, err := LoadConfig([]string{"cmd"})
	if err == nil {
		t.Fatal("expected error when model is missing")
	}
}

func TestLoadConfigPromptRequired(t *testing.T) {
	t.Setenv("TRELLO_API_SECRET", "env-secret")
	t.Setenv("CALLBACK_URL", "https://env.example.com/trello")
	t.Setenv("FORWARD_URL", "http://127.0.0.1:18789/hooks/agent")
	t.Setenv("FORWARD_TOKEN", "env-token")
	t.Setenv("MODEL", "copilot-api/claude-haiku-4.5")

	_, err := LoadConfig([]string{"cmd"})
	if err == nil {
		t.Fatal("expected error when prompt is missing")
	}
}
