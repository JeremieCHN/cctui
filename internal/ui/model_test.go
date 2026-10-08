package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"cctui/internal/ccswitch"
)

var claudeFormValues = []string{
	"Relay",                     // Name
	"https://relay.example.com", // Base URL
	"sk-test",                   // API Key
	"relay-main",                // Model
	"",                          // Fable Model
	"relay-haiku",               // Haiku Model
	"",                          // Sonnet Model
	"relay-opus",                // Opus Model
	"relay-subagent",            // Subagent Model
	"",                          // Website
	"",                          // Notes
}

func TestClaudeFormSavesMultipleModels(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CC_SWITCH_TEST_HOME", home)

	store, err := ccswitch.OpenStore()
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	m, err := NewModel(store, nil)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}

	m.openAddForm(ccswitch.AppClaude)
	if got := len(m.form.fields); got != len(claudeFormValues) {
		t.Fatalf("表单字段数 = %d, want %d", got, len(claudeFormValues))
	}
	for index, value := range claudeFormValues {
		m.form.fields[index].SetValue(value)
	}

	if _, _ = m.saveForm(); m.mode != modeList {
		t.Fatalf("保存失败: %s", m.form.errorMessage)
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
		"ANTHROPIC_MODEL":                     "relay-main",
		"ANTHROPIC_DEFAULT_FABLE_MODEL":       "relay-main",
		"ANTHROPIC_DEFAULT_FABLE_MODEL_NAME":  "relay-main",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":       "relay-haiku",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME":  "relay-haiku",
		"ANTHROPIC_DEFAULT_SONNET_MODEL":      "relay-main",
		"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME": "relay-main",
		"ANTHROPIC_DEFAULT_OPUS_MODEL":        "relay-opus",
		"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME":   "relay-opus",
		"CLAUDE_CODE_SUBAGENT_MODEL":          "relay-subagent",
	}
	for key, want := range wantEnv {
		got, _ := env[key].(string)
		if got != want {
			t.Errorf("env[%s] = %q, want %q", key, got, want)
		}
	}

	providers := m.providers[ccswitch.AppClaude]
	if len(providers) != 1 {
		t.Fatalf("供应商数量 = %d, want 1", len(providers))
	}

	// 再次编辑时应回显实际写入的值，留空的别名字段已按继承回填
	m.openEditForm(ccswitch.AppClaude, providers[0])
	wantFormValues := []string{
		"Relay",
		"https://relay.example.com",
		"sk-test",
		"relay-main",
		"relay-main",
		"relay-haiku",
		"relay-main",
		"relay-opus",
		"relay-subagent",
		"",
		"",
	}
	got := make([]string, 0, len(m.form.fields))
	for _, field := range m.form.fields {
		got = append(got, field.Value())
	}
	for index, want := range wantFormValues {
		if got[index] != want {
			t.Errorf("字段 %s 回显 = %q, want %q", m.form.labels[index], got[index], want)
		}
	}
}

func TestFormWindowKeepsFocusedFieldVisible(t *testing.T) {
	m := &Model{
		width:     90,
		height:    20,
		mode:      modeForm,
		providers: map[ccswitch.AppType][]ccswitch.Provider{},
		current:   map[ccswitch.AppType]string{},
	}
	m.form = newFormState(ccswitch.AppClaude, nil, ccswitch.ProviderInput{})

	for index := range m.form.fields {
		m.form.focusIndex = index
		m.form.syncFocus()

		view := m.View()
		if lines := len(strings.Split(view, "\n")); lines != m.height {
			t.Fatalf("焦点 %d: 视图 %d 行, want %d", index, lines, m.height)
		}
		if !strings.Contains(view, labelStyle.Render(m.form.labels[index])) {
			t.Fatalf("焦点 %d: 字段 %q 不在可见窗口内", index, m.form.labels[index])
		}
	}

	m.form.focusIndex = len(m.form.fields) - 1
	m.form.syncFocus()
	if view := m.View(); strings.Contains(view, labelStyle.Render(m.form.labels[0])) {
		t.Error("聚焦最后一个字段时，首个字段应已滚出窗口")
	}
}

func newPickerTestModel() *Model {
	m := &Model{
		width:     100,
		height:    30,
		mode:      modeForm,
		providers: map[ccswitch.AppType][]ccswitch.Provider{},
		current:   map[ccswitch.AppType]string{},
	}
	m.form = newFormState(ccswitch.AppClaude, nil, ccswitch.ProviderInput{})
	return m
}

func ctrlL() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlL}
}

func TestModelPickerShortcutOnlyOnModelFields(t *testing.T) {
	m := newPickerTestModel()

	m.form.focusIndex = 0
	if _, _ = m.updateForm(ctrlL()); m.mode != modeForm || m.form.errorMessage == "" {
		t.Fatalf("非模型字段不应打开选择器, mode = %v, err = %q", m.mode, m.form.errorMessage)
	}

	m.form.focusIndex = 5 // Haiku Model
	m.form.errorMessage = ""
	_, cmd := m.updateForm(ctrlL())
	if m.mode != modeModelPicker {
		t.Fatalf("模型字段应打开选择器, mode = %v", m.mode)
	}
	if m.picker == nil || m.picker.fieldIndex != 5 || m.picker.fieldLabel != "Haiku Model" {
		t.Fatalf("picker 目标字段不对: %+v", m.picker)
	}
	if !m.picker.loading || cmd == nil {
		t.Error("首次打开应进入加载状态并返回拉取命令")
	}
}

func TestModelPickerFilterAndSelection(t *testing.T) {
	m := newPickerTestModel()
	m.form.fields[3].SetValue("relay-sonnet-4-5")
	m.form.focusIndex = 3

	if _, cmd := m.updateForm(ctrlL()); cmd == nil {
		t.Fatal("应返回拉取命令")
	}
	m.Update(modelOptionsMsg{
		options: []string{"relay-sonnet-4-5", "relay-opus-4-5", "relay-haiku-4-5"},
		source:  "http://relay.example.com/v1/models",
	})

	if m.picker.loading {
		t.Error("拿到结果后应结束加载状态")
	}
	if got := m.picker.filtered; len(got) != 3 {
		t.Fatalf("过滤后数量 = %d, want 3", len(got))
	}
	if m.picker.cursor != 0 {
		t.Errorf("光标应停在当前字段的值上, cursor = %d", m.picker.cursor)
	}
	if view := m.View(); !strings.Contains(view, "共 3 个模型") {
		t.Error("选择器应显示模型数量")
	}

	for _, r := range "opus" {
		_, _ = m.updateModelPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if got := m.picker.filtered; len(got) != 1 || got[0] != "relay-opus-4-5" {
		t.Fatalf("筛选结果 = %v, want [relay-opus-4-5]", got)
	}

	if _, _ = m.updateModelPicker(tea.KeyMsg{Type: tea.KeyEnter}); m.mode != modeForm {
		t.Fatalf("选择后应回到表单, mode = %v", m.mode)
	}
	if got := m.form.fields[3].Value(); got != "relay-opus-4-5" {
		t.Errorf("字段值 = %q, want relay-opus-4-5", got)
	}
	if m.picker != nil {
		t.Error("选择后应清理 picker 状态")
	}
}

func TestModelPickerReusesFetchedList(t *testing.T) {
	m := newPickerTestModel()
	m.form.focusIndex = 3
	if _, _ = m.updateForm(ctrlL()); m.mode != modeModelPicker {
		t.Fatal("应打开选择器")
	}
	m.Update(modelOptionsMsg{options: []string{"m1"}, source: "http://relay.example.com/v1/models"})

	if _, _ = m.updateModelPicker(tea.KeyMsg{Type: tea.KeyEsc}); m.mode != modeForm {
		t.Fatal("Esc 应回到表单")
	}

	_, cmd := m.updateForm(ctrlL())
	if m.picker == nil || m.picker.loading {
		t.Fatal("有缓存时再次打开不应重新拉取")
	}
	if cmd == nil {
		t.Error("再次打开仍应返回光标闪烁命令")
	}
	if len(m.picker.filtered) != 1 {
		t.Errorf("应直接复用缓存列表, got %v", m.picker.filtered)
	}
}

func TestModelPickerShowsErrorAndKeepsHeight(t *testing.T) {
	m := newPickerTestModel()
	m.height = 20
	m.form.focusIndex = 3
	if _, _ = m.updateForm(ctrlL()); m.mode != modeModelPicker {
		t.Fatal("应打开选择器")
	}

	m.Update(modelOptionsMsg{err: errors.New("connection refused")})
	if view := m.View(); !strings.Contains(view, "connection refused") || !strings.Contains(view, "Ctrl+R") {
		t.Error("失败时应显示错误与重试提示")
	}
	if lines := len(strings.Split(m.View(), "\n")); lines != m.height {
		t.Fatalf("错误态视图 %d 行, want %d", lines, m.height)
	}

	options := make([]string, 0, 60)
	for index := 0; index < 60; index++ {
		options = append(options, fmt.Sprintf("relay-model-%02d", index))
	}
	m.Update(modelOptionsMsg{options: options, source: "http://relay.example.com/v1/models"})

	for index := 0; index < len(options); index++ {
		_, _ = m.updateModelPicker(tea.KeyMsg{Type: tea.KeyDown})
	}
	view := m.View()
	if lines := len(strings.Split(view, "\n")); lines != m.height {
		t.Fatalf("列表态视图 %d 行, want %d", lines, m.height)
	}
	if !strings.Contains(view, options[len(options)-1]) {
		t.Error("光标移到底部时最后一项应可见")
	}
	if strings.Contains(view, options[0]) {
		t.Error("光标移到底部时第一项应滚出窗口")
	}
}
