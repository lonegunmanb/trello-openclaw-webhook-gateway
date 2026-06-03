package router

import (
	"errors"
	"fmt"
	"strings"
)

var supportedProviderTypes = map[string]struct{}{
	"openai":    {},
	"azure":     {},
	"anthropic": {},
}

var supportedWireAPIs = map[string]struct{}{
	"completions": {},
	"responses":   {},
}

// ProviderConfig configures rule-level BYOK routing for one worker session.
// A nil *ProviderConfig on RuleMatch means the SDK should keep its default
// Copilot routing for that matched rule.
type ProviderConfig struct {
	Type            string `hcl:"type,optional"`
	BaseURL         string `hcl:"base_url,optional"`
	APIKey          string `hcl:"api_key,optional"`
	APIKeyRef       string `hcl:"api_key_ref,optional"`
	BearerToken     string `hcl:"bearer_token,optional"`
	WireAPI         string `hcl:"wire_api,optional"`
	AzureAPIVersion string `hcl:"azure_api_version,optional"`
}

func (provider *ProviderConfig) normalize() {
	provider.Type = strings.TrimSpace(provider.Type)
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.APIKeyRef = strings.TrimSpace(provider.APIKeyRef)
	provider.WireAPI = strings.TrimSpace(provider.WireAPI)
	provider.AzureAPIVersion = strings.TrimSpace(provider.AzureAPIVersion)
}

// IsEnabled reports whether any BYOK field has been set.
func (provider ProviderConfig) IsEnabled() bool {
	return provider.Type != "" ||
		provider.BaseURL != "" ||
		provider.APIKey != "" ||
		provider.APIKeyRef != "" ||
		provider.BearerToken != "" ||
		provider.WireAPI != "" ||
		provider.AzureAPIVersion != ""
}

func (provider ProviderConfig) validate() error {
	if !provider.IsEnabled() {
		return nil
	}
	if provider.Type == "" && provider.BaseURL == "" {
		return fmt.Errorf("provider field %q requires type and base_url to be set", provider.firstAuxiliaryField())
	}
	if provider.Type == "" || provider.BaseURL == "" {
		return errors.New("provider type and base_url must both be set or both empty")
	}
	if _, ok := supportedProviderTypes[provider.Type]; !ok {
		return fmt.Errorf("unsupported provider type %q (want openai, azure, or anthropic)", provider.Type)
	}
	if provider.APIKey != "" && provider.APIKeyRef != "" {
		return errors.New("provider api_key and api_key_ref are mutually exclusive")
	}
	if provider.AzureAPIVersion != "" && provider.Type != "azure" {
		return errors.New("provider azure_api_version requires type = \"azure\"")
	}
	if provider.WireAPI != "" {
		if _, ok := supportedWireAPIs[provider.WireAPI]; !ok {
			return fmt.Errorf("unsupported provider wire_api %q (want completions or responses)", provider.WireAPI)
		}
	}
	return nil
}

func (provider ProviderConfig) firstAuxiliaryField() string {
	switch {
	case provider.APIKey != "":
		return "api_key"
	case provider.APIKeyRef != "":
		return "api_key_ref"
	case provider.BearerToken != "":
		return "bearer_token"
	case provider.WireAPI != "":
		return "wire_api"
	case provider.AzureAPIVersion != "":
		return "azure_api_version"
	default:
		return "provider"
	}
}

func (provider *ProviderConfig) resolveAPIKey(lookup func(string) (string, bool)) error {
	if provider.APIKeyRef == "" {
		return nil
	}
	if lookup == nil {
		return errors.New("provider api_key_ref lookup is nil")
	}
	value, ok := lookup(provider.APIKeyRef)
	if !ok || value == "" {
		return fmt.Errorf("provider api_key_ref %q is unset or empty", provider.APIKeyRef)
	}
	provider.APIKey = value
	return nil
}

func cloneProvider(provider *ProviderConfig) *ProviderConfig {
	if provider == nil {
		return nil
	}
	cloned := *provider
	return &cloned
}
