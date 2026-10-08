package ccswitch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestModelListEndpoints(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    []string
	}{
		{
			name:    "裸域名",
			baseURL: "https://relay.example.com",
			want:    []string{"https://relay.example.com/v1/models", "https://relay.example.com/api/pricing"},
		},
		{
			name:    "带 v1 与尾部斜杠",
			baseURL: "https://relay.example.com/v1/",
			want:    []string{"https://relay.example.com/v1/models", "https://relay.example.com/api/pricing"},
		},
		{
			name:    "缺少 scheme",
			baseURL: "relay.example.com",
			want:    []string{"https://relay.example.com/v1/models", "https://relay.example.com/api/pricing"},
		},
		{
			name:    "带子路径",
			baseURL: "https://relay.example.com/proxy/",
			want:    []string{"https://relay.example.com/proxy/v1/models", "https://relay.example.com/proxy/api/pricing"},
		},
		{
			name:    "空值",
			baseURL: "   ",
			want:    nil,
		},
		{
			name:    "没有 host",
			baseURL: "https:///v1",
			want:    nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ModelListEndpoints(tc.baseURL); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ModelListEndpoints(%q) = %v, want %v", tc.baseURL, got, tc.want)
			}
		})
	}
}

func TestParseModelNames(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "OpenAI 风格",
			body: `{"object":"list","data":[{"id":"gpt-4o","object":"model"},{"id":"claude-sonnet-4-5"}]}`,
			want: []string{"claude-sonnet-4-5", "gpt-4o"},
		},
		{
			name: "字符串数组",
			body: `{"data":["b-model","a-model"]}`,
			want: []string{"a-model", "b-model"},
		},
		{
			name: "new-api pricing",
			body: `{"success":true,"data":[{"model_name":"relay-opus-4-5","quota_type":0},{"model_name":"relay-haiku-4-5"}]}`,
			want: []string{"relay-haiku-4-5", "relay-opus-4-5"},
		},
		{
			name: "models 字段",
			body: `{"models":[{"id":"m1"},{"id":"m2"}]}`,
			want: []string{"m1", "m2"},
		},
		{
			name: "顶层数组",
			body: `[{"id":"m1"},"m2"]`,
			want: []string{"m1", "m2"},
		},
		{
			name: "去重排序并忽略空白",
			body: `{"data":[{"id":" b "},{"id":"a"},{"id":"a"},{"id":"  "}]}`,
			want: []string{"a", "b"},
		},
		{
			name: "无关结构",
			body: `{"success":false,"message":"unauthorized"}`,
			want: nil,
		},
		{
			name: "非 JSON",
			body: `<html>502</html>`,
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseModelNames([]byte(tc.body)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseModelNames() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFetchModelOptionsUsesV1Models(t *testing.T) {
	var gotAuth, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotAuth = request.Header.Get("Authorization")
		gotPath = request.URL.Path
		_, _ = writer.Write([]byte(`{"data":[{"id":"relay-opus-4-5"}]}`))
	}))
	defer server.Close()

	options, source, warning, err := FetchModelOptions(context.Background(), server.URL, "sk-test")
	if err != nil {
		t.Fatalf("FetchModelOptions: %v", err)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer sk-test")
	}
	if gotPath != "/v1/models" {
		t.Errorf("请求路径 = %q, want /v1/models", gotPath)
	}
	if !reflect.DeepEqual(options, []string{"relay-opus-4-5"}) {
		t.Errorf("options = %v", options)
	}
	if source != server.URL+"/v1/models" {
		t.Errorf("source = %q, want %q", source, server.URL+"/v1/models")
	}
	if warning != "" {
		t.Errorf("warning = %q, want 空", warning)
	}
}

func TestFetchModelOptionsFallsBackToPricing(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		if request.URL.Path == "/v1/models" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = writer.Write([]byte(`{"success":true,"data":[{"model_name":"relay-model"}]}`))
	}))
	defer server.Close()

	options, source, warning, err := FetchModelOptions(context.Background(), server.URL, "sk-test")
	if err != nil {
		t.Fatalf("FetchModelOptions: %v", err)
	}
	if !reflect.DeepEqual(paths, []string{"/v1/models", "/api/pricing"}) {
		t.Errorf("请求顺序 = %v", paths)
	}
	if !reflect.DeepEqual(options, []string{"relay-model"}) {
		t.Errorf("options = %v", options)
	}
	if source != server.URL+"/api/pricing" {
		t.Errorf("source = %q", source)
	}
	if warning == "" {
		t.Error("回退时应有提示")
	}
}

func TestFetchModelOptionsWithoutKey(t *testing.T) {
	var gotAuth string
	var hasAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotAuth = request.Header.Get("Authorization")
		_, hasAuth = request.Header["Authorization"]
		_, _ = writer.Write([]byte(`{"data":[{"id":"m1"}]}`))
	}))
	defer server.Close()

	if _, _, _, err := FetchModelOptions(context.Background(), server.URL, "  "); err != nil {
		t.Fatalf("FetchModelOptions: %v", err)
	}
	if hasAuth || gotAuth != "" {
		t.Errorf("未配置 key 时不应发送 Authorization, got %q", gotAuth)
	}
}

func TestFetchModelOptionsErrors(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	if _, _, _, err := FetchModelOptions(context.Background(), server.URL, "sk-test"); err == nil {
		t.Error("两个接口都失败时应返回错误")
	} else if message := err.Error(); !strings.Contains(message, "/v1/models") || !strings.Contains(message, "/api/pricing 同样不可用") {
		t.Errorf("错误信息应说明两个接口都试过, got %q", message)
	}
	if requests != 2 {
		t.Errorf("请求次数 = %d, want 2", requests)
	}

	if _, _, _, err := FetchModelOptions(context.Background(), "", "sk-test"); err == nil {
		t.Error("Base URL 为空时应返回错误")
	}
}
