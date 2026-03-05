package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"github.com/ollama/ollama/envconfig"
)

// ProviderConfig represents the configuration for a specific AI provider
type ProviderConfig struct {
	ID                   string      `json:"id"`
	Name                 string      `json:"name"`
	BaseURL              string      `json:"base_url"`
	Type                 string      `json:"type"`
	APIKey               string      `json:"api_key"`
	OAuth                  interface{} `json:"oauth,omitempty"`
	Disable              bool        `json:"disable,omitempty"`
	SystemPromptPrefix   string      `json:"system_prompt_prefix,omitempty"`
	ExtraHeaders         interface{} `json:"extra_headers,omitempty"`
	ExtraBody            interface{} `json:"extra_body,omitempty"`
	ProviderOptions      interface{} `json:"provider_options,omitempty"`
	Models               []Model     `json:"models,omitempty"`
}

// Model represents an AI model configuration
type Model struct {
	ID                      string      `json:"id"`
	Name                    string      `json:"name"`
	CostPer1MIn             float64     `json:"cost_per_1m_in"`
	CostPer1MOut            float64     `json:"cost_per_1m_out"`
	CostPer1MInCached       float64     `json:"cost_per_1m_in_cached"`
	CostPer1MOutCached      float64     `json:"cost_per_1m_out_cached"`
	ContextWindow           int         `json:"context_window"`
	DefaultMaxTokens        int         `json:"default_max_tokens"`
	CanReason               bool        `json:"can_reason"`
	ReasoningLevels         []string    `json:"reasoning_levels,omitempty"`
	DefaultReasoningEffort  string      `json:"default_reasoning_effort,omitempty"`
	SupportsAttachments     bool        `json:"supports_attachments"`
	Options                 ModelOptions `json:"options"`
}

// ModelOptions represents options for a specific model
type ModelOptions struct {
	Temperature     float64      `json:"temperature,omitempty"`
	TopP            float64      `json:"top_p,omitempty"`
	TopK            int          `json:"top_k,omitempty"`
	FrequencyPenalty float64     `json:"frequency_penalty,omitempty"`
	PresencePenalty float64     `json:"presence_penalty,omitempty"`
	ProviderOptions interface{}  `json:"provider_options,omitempty"`
}

// Config represents the main configuration structure
type Config struct {
	Schema     string              `json:"$schema"`
	ID         string              `json:"$id"`
	Ref        string              `json:"$ref"`
	Defs       interface{}         `json:"$defs,omitempty"`
	Models     map[string]Model    `json:"models,omitempty"`
	Providers  map[string]ProviderConfig `json:"providers,omitempty"`
	MCP        interface{}         `json:"mcp,omitempty"`
	LSP        interface{}         `json:"lsp,omitempty"`
	Options    interface{}         `json:"options,omitempty"`
	Permissions interface{}        `json:"permissions,omitempty"`
	Tools      interface{}         `json:"tools,omitempty"`
}

// ReadConfig reads the configuration from a JSON file into a Config struct
func ReadConfig(filename string) (*Config, error) {
	// Read file content
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		// If file does not exist, return empty Config structure
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON into Config struct
	var p Config
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &p, nil
}

// WriteConfig writes the configuration from a Config struct to a JSON file
func WriteConfig(config *Config, filename string) error {
	// Marshal the config struct to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write the JSON data to file
	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// AddProvider adds a new provider to the configuration
func (c *Config) AddProvider(id string, provider ProviderConfig) {
	if c.Providers == nil {
		c.Providers = make(map[string]ProviderConfig)
	}
	c.Providers[id] = provider
}

// GetProvider retrieves a provider by ID
func (c *Config) GetProvider(id string) (*ProviderConfig, bool) {
	if c.Providers == nil {
		return nil, false
	}
	provider, exists := c.Providers[id]
	return &provider, exists
}

// GetProviderOrDefault retrieves a provider by ID or returns a default provider if not found
func (c *Config) GetProviderOrDefault(id string, defaultProvider ProviderConfig) (*ProviderConfig, bool) {
	if c.Providers == nil {
		return &defaultProvider, false
	}
	provider, exists := c.Providers[id]
	if !exists {
		return &defaultProvider, false
	}
	return &provider, true
}

// AddModel adds a new model to a specific provider
func (c *Config) AddModel(providerID string, model Model) error {
	provider, exists := c.GetProvider(providerID)
	if !exists {
		return fmt.Errorf("provider %s not found", providerID)
	}
	
	// Check if model with same ID already exists
	if provider.Models != nil {
		for _, existingModel := range provider.Models {
			if existingModel.ID == model.ID {
				return nil
			}
		}
	}
	
	if provider.Models == nil {
		provider.Models = []Model{}
	}
	provider.Models = append(provider.Models, model)
	c.Providers[providerID] = *provider
	return nil
}

// Crush implements Runner and Editor for Crush integration
type Crush struct{}

func (o *Crush) String() string { return "Crush" }

func (o *Crush) Run(model string, args []string) error {
	if _, err := exec.LookPath("crush"); err != nil {
		return fmt.Errorf("crush is not installed, install from https://charm.land")
	}

	// Call Edit() to ensure config is up-to-date before launch
	models := []string{model}
	if config, err := loadIntegration("crush"); err == nil && len(config.Models) > 0 {
		models = config.Models
	}
	if err := o.Edit(models); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	cmd := exec.Command("crush", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (o *Crush) Paths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var paths []string
	p := filepath.Join(home, ".config", "crush", "crush.json")
	if _, err := os.Stat(p); err == nil {
		paths = append(paths, p)
	}
	sp := filepath.Join(home, ".local", "state", "crush", "model.json")
	if _, err := os.Stat(sp); err == nil {
		paths = append(paths, sp)
	}
	return paths
}

func (o *Crush) Edit(modelList []string) error {
	if len(modelList) == 0 {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(home, ".config", "crush", "crush.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	config, err := ReadConfig(configPath)
	if err != nil {
		return err
	}

	config.Schema = "https://charm.land/crush.json"

	defaultProvider := ProviderConfig{
		ID:      "ollama",
		Name:    "Ollama (local)",
		BaseURL: envconfig.Host().String() + "/v1",
		Type:    "openai-compat",
	}

	config.GetProviderOrDefault("ollama", defaultProvider)

	for _, model := range modelList {
		fmt.Printf("model: %s\n", model)
		newModel := Model{
			ID:model,
		}
		err = config.AddModel("ollama", newModel)
		if err != nil {
			log.Printf("Error adding model: %v", err)
		} else {
			fmt.Println("Model added successfully to provider")
		}
	}



	err = WriteConfig(config, configPath)
	if err != nil {
		log.Fatal(err)
	}
	return err
}

func (o *Crush) Models() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	config, err := readJSONFile(filepath.Join(home, ".config", "crush", "crush.json"))
	if err != nil {
		return nil
	}
	provider, _ := config["provider"].(map[string]any)
	ollama, _ := provider["ollama"].(map[string]any)
	models, _ := ollama["models"].(map[string]any)
	if len(models) == 0 {
		return nil
	}
	keys := slices.Collect(maps.Keys(models))
	slices.Sort(keys)
	return keys
}

