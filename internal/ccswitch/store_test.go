package ccswitch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

var claudeModelKeys = []string{
	"ANTHROPIC_MODEL",
	"ANTHROPIC_DEFAULT_FABLE_MODEL",
	"ANTHROPIC_DEFAULT_FABLE_MODEL_NAME",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME",
	"ANTHROPIC_DEFAULT_SONNET_MODEL",
	"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME",
	"ANTHROPIC_DEFAULT_OPUS_MODEL",
	"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME",
	"CLAUDE_CODE_SUBAGENT_MODEL",
}

func claudeModelsEqual(value string) map[string]string {
	want := map[string]string{}
	for _, key := range claudeModelKeys {
		want[key] = value
	}
	return want
}

func TestBuildProviderClaudeModels(t *testing.T) {
	store := &Store{}

	cases := []struct {
		name     string
		existing *Provider
		input    ProviderInput
		want     map[string]string
	}{
		{
			name:  "all empty removes every model env",
			input: ProviderInput{Name: "relay"},
			want:  map[string]string{},
		},
		{
			name:  "main model only inherits to aliases",
			input: ProviderInput{Name: "relay", Model: "claude-sonnet-4-5"},
			want:  claudeModelsEqual("claude-sonnet-4-5"),
		},
		{
			name: "explicit aliases override main model",
			input: ProviderInput{
				Name:       "relay",
				Model:      "claude-sonnet-4-5",
				ModelFable: "claude-fable-4",
				ModelOpus:  "claude-opus-4-5",
			},
			want: map[string]string{
				"ANTHROPIC_MODEL":                     "claude-sonnet-4-5",
				"ANTHROPIC_DEFAULT_FABLE_MODEL":       "claude-fable-4",
				"ANTHROPIC_DEFAULT_FABLE_MODEL_NAME":  "claude-fable-4",
				"ANTHROPIC_DEFAULT_HAIKU_MODEL":       "claude-sonnet-4-5",
				"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME":  "claude-sonnet-4-5",
				"ANTHROPIC_DEFAULT_SONNET_MODEL":      "claude-sonnet-4-5",
				"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME": "claude-sonnet-4-5",
				"ANTHROPIC_DEFAULT_OPUS_MODEL":        "claude-opus-4-5",
				"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME":   "claude-opus-4-5",
				"CLAUDE_CODE_SUBAGENT_MODEL":          "claude-sonnet-4-5",
			},
		},
		{
			name: "subagent model overrides main model",
			input: ProviderInput{
				Name:          "relay",
				Model:         "relay-main",
				ModelSubagent: "relay-subagent",
			},
			want: map[string]string{
				"ANTHROPIC_MODEL":                     "relay-main",
				"ANTHROPIC_DEFAULT_FABLE_MODEL":       "relay-main",
				"ANTHROPIC_DEFAULT_FABLE_MODEL_NAME":  "relay-main",
				"ANTHROPIC_DEFAULT_HAIKU_MODEL":       "relay-main",
				"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME":  "relay-main",
				"ANTHROPIC_DEFAULT_SONNET_MODEL":      "relay-main",
				"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME": "relay-main",
				"ANTHROPIC_DEFAULT_OPUS_MODEL":        "relay-main",
				"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME":   "relay-main",
				"CLAUDE_CODE_SUBAGENT_MODEL":          "relay-subagent",
			},
		},
		{
			name:  "alias without main model only writes that alias",
			input: ProviderInput{Name: "relay", ModelSonnet: "relay-sonnet"},
			want: map[string]string{
				"ANTHROPIC_DEFAULT_SONNET_MODEL":      "relay-sonnet",
				"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME": "relay-sonnet",
			},
		},
		{
			name: "cleared alias falls back to main model",
			existing: &Provider{SettingsConfig: map[string]any{"env": map[string]any{
				"ANTHROPIC_MODEL":              "relay-main",
				"ANTHROPIC_DEFAULT_OPUS_MODEL": "relay-opus",
			}}},
			input: ProviderInput{Name: "relay", Model: "relay-main"},
			want:  claudeModelsEqual("relay-main"),
		},
		{
			name: "empty main model removes stale alias",
			existing: &Provider{SettingsConfig: map[string]any{"env": map[string]any{
				"ANTHROPIC_MODEL":               "relay-main",
				"ANTHROPIC_DEFAULT_HAIKU_MODEL": "relay-haiku",
			}}},
			input: ProviderInput{Name: "relay"},
			want:  map[string]string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := store.buildProvider(AppClaude, tc.existing, tc.input, "relay")
			if err != nil {
				t.Fatalf("buildProvider: %v", err)
			}

			env, _ := provider.SettingsConfig["env"].(map[string]any)
			for _, key := range claudeModelKeys {
				got := stringValue(env[key])
				if want := tc.want[key]; got != want {
					t.Errorf("env[%s] = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestExtractInputClaudeModels(t *testing.T) {
	store := &Store{}

	cases := []struct {
		name string
		env  map[string]any
		want [6]string
	}{
		{
			name: "aliases equal to main model are shown as configured",
			env: map[string]any{
				"ANTHROPIC_MODEL":                    "relay-main",
				"ANTHROPIC_DEFAULT_FABLE_MODEL":      "relay-main",
				"ANTHROPIC_DEFAULT_FABLE_MODEL_NAME": "relay-main",
				"ANTHROPIC_DEFAULT_HAIKU_MODEL":      "relay-main",
				"ANTHROPIC_DEFAULT_SONNET_MODEL":     "relay-main",
				"ANTHROPIC_DEFAULT_OPUS_MODEL":       "relay-main",
				"CLAUDE_CODE_SUBAGENT_MODEL":         "relay-main",
			},
			want: [6]string{"relay-main", "relay-main", "relay-main", "relay-main", "relay-main", "relay-main"},
		},
		{
			name: "explicit alias is shown",
			env: map[string]any{
				"ANTHROPIC_MODEL":              "relay-main",
				"ANTHROPIC_DEFAULT_OPUS_MODEL": "relay-opus",
			},
			want: [6]string{"relay-main", "", "", "", "relay-opus", ""},
		},
		{
			name: "alias without main model is shown",
			env: map[string]any{
				"ANTHROPIC_DEFAULT_SONNET_MODEL": "relay-sonnet",
			},
			want: [6]string{"", "", "", "relay-sonnet", "", ""},
		},
		{
			name: "subagent model is shown",
			env: map[string]any{
				"ANTHROPIC_MODEL":            "relay-main",
				"CLAUDE_CODE_SUBAGENT_MODEL": "relay-subagent",
			},
			want: [6]string{"relay-main", "", "", "", "", "relay-subagent"},
		},
		{
			name: "no model configured",
			env:  map[string]any{},
			want: [6]string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := Provider{
				ID:             "relay",
				Name:           "relay",
				SettingsConfig: map[string]any{"env": tc.env},
			}
			input := store.ExtractInput(AppClaude, provider)
			got := [6]string{input.Model, input.ModelFable, input.ModelHaiku, input.ModelSonnet, input.ModelOpus, input.ModelSubagent}
			if got != tc.want {
				t.Errorf("models = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClaudeProviderWritesLiveSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CC_SWITCH_TEST_HOME", home)

	store, err := OpenStore()
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	if _, _, err := store.AddProvider(AppClaude, ProviderInput{
		Name:      "Relay",
		BaseURL:   "https://relay.example.com",
		APIKey:    "sk-test",
		Model:     "relay-main",
		ModelOpus: "relay-opus",
	}); err != nil {
		t.Fatalf("AddProvider: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("读取 live 配置: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(content, &doc); err != nil {
		t.Fatalf("解析 live 配置: %v", err)
	}
	env, _ := doc["env"].(map[string]any)

	wantEnv := map[string]string{
		"ANTHROPIC_BASE_URL":                  "https://relay.example.com",
		"ANTHROPIC_AUTH_TOKEN":                "sk-test",
		"ANTHROPIC_MODEL":                     "relay-main",
		"ANTHROPIC_DEFAULT_FABLE_MODEL":       "relay-main",
		"ANTHROPIC_DEFAULT_FABLE_MODEL_NAME":  "relay-main",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":       "relay-main",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME":  "relay-main",
		"ANTHROPIC_DEFAULT_SONNET_MODEL":      "relay-main",
		"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME": "relay-main",
		"ANTHROPIC_DEFAULT_OPUS_MODEL":        "relay-opus",
		"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME":   "relay-opus",
		"CLAUDE_CODE_SUBAGENT_MODEL":          "relay-main",
	}
	for key, want := range wantEnv {
		got, _ := env[key].(string)
		if got != want {
			t.Errorf("env[%s] = %q, want %q", key, got, want)
		}
	}
}

func TestBuildProviderClaudePreservesUnmanagedEnv(t *testing.T) {
	store := &Store{}
	existing := &Provider{SettingsConfig: map[string]any{"env": map[string]any{
		"ANTHROPIC_SMALL_FAST_MODEL": "claude-3-5-haiku",
		"ANTHROPIC_CUSTOM_HEADERS":   "x-trace: 1",
	}}}

	provider, err := store.buildProvider(AppClaude, existing, ProviderInput{Name: "relay", Model: "relay-main"}, "relay")
	if err != nil {
		t.Fatalf("buildProvider: %v", err)
	}

	env, _ := provider.SettingsConfig["env"].(map[string]any)
	for key, want := range map[string]string{
		"ANTHROPIC_SMALL_FAST_MODEL": "claude-3-5-haiku",
		"ANTHROPIC_CUSTOM_HEADERS":   "x-trace: 1",
	} {
		if got := stringValue(env[key]); got != want {
			t.Errorf("env[%s] = %q, want %q", key, got, want)
		}
	}
}

func TestBootstrapSyncsCurrentProviderFromLive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CC_SWITCH_TEST_HOME", home)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	writeLive := func(model string) {
		t.Helper()
		buf, err := json.Marshal(map[string]any{"env": map[string]any{"ANTHROPIC_MODEL": model}})
		if err != nil {
			t.Fatalf("序列化 live 配置: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
			t.Fatalf("创建目录: %v", err)
		}
		if err := os.WriteFile(settingsPath, buf, 0o644); err != nil {
			t.Fatalf("写入 live 配置: %v", err)
		}
	}

	store, err := OpenStore()
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	writeLive("relay-a")
	if _, err := store.Bootstrap(); err != nil {
		t.Fatalf("首次 Bootstrap: %v", err)
	}

	writeLive("relay-b")
	warnings, err := store.Bootstrap()
	if err != nil {
		t.Fatalf("第二次 Bootstrap: %v", err)
	}
	if len(warnings) == 0 {
		t.Error("live 变化时应返回同步提示")
	}

	providers, err := store.ListProviders(AppClaude)
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("供应商数量 = %d, want 1", len(providers))
	}
	env, _ := providers[0].SettingsConfig["env"].(map[string]any)
	if got := stringValue(env["ANTHROPIC_MODEL"]); got != "relay-b" {
		t.Errorf("同步后 ANTHROPIC_MODEL = %q, want relay-b", got)
	}
	if providers[0].Name != "Imported Claude" {
		t.Errorf("同步不应改变供应商名称, got %q", providers[0].Name)
	}

	warnings, err = store.Bootstrap()
	if err != nil {
		t.Fatalf("第三次 Bootstrap: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("无变更时不应有提示, got %v", warnings)
	}

	if err := os.Remove(settingsPath); err != nil {
		t.Fatalf("删除 live 配置: %v", err)
	}
	if _, err := store.Bootstrap(); err != nil {
		t.Fatalf("第四次 Bootstrap: %v", err)
	}
	providers, _ = store.ListProviders(AppClaude)
	env, _ = providers[0].SettingsConfig["env"].(map[string]any)
	if got := stringValue(env["ANTHROPIC_MODEL"]); got != "relay-b" {
		t.Errorf("live 缺失时记录不应被改动, got %q", got)
	}
}
