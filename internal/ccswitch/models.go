package ccswitch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	modelListTimeout  = 10 * time.Second
	modelListMaxBytes = 4 << 20
)

// NormalizeBaseURL 去掉空白与尾部斜杠，缺少 scheme 时补 https://。
func NormalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	return trimmed
}

// ModelListEndpoints 返回按优先级排列的模型列表接口：
// 先试 OpenAI 兼容的 /v1/models（返回当前 key 可用的模型），
// 再试 new-api 模型广场的 /api/pricing（公开接口，不需要 key）。
func ModelListEndpoints(baseURL string) []string {
	base := NormalizeBaseURL(baseURL)
	if base == "" {
		return nil
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" {
		return nil
	}

	root := base
	primary := base + "/v1/models"
	if strings.HasSuffix(base, "/v1") {
		root = strings.TrimSuffix(base, "/v1")
		primary = base + "/models"
	}

	return []string{primary, root + "/api/pricing"}
}

// ParseModelNames 从响应体中提取模型名，兼容：
//
//	{"data":[{"id":"gpt-4o"}, ...]}          OpenAI 兼容
//	{"data":["gpt-4o", ...]}
//	{"data":[{"model_name":"gpt-4o"}, ...]}  new-api /api/pricing
//
// 结果去重并升序排序。
func ParseModelNames(body []byte) []string {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}

	var names []string
	collectModelNames(payload, &names)
	return normalizeModelNames(names)
}

func collectModelNames(node any, out *[]string) {
	switch typed := node.(type) {
	case []any:
		collectModelList(typed, out)
	case map[string]any:
		for key, value := range typed {
			switch key {
			case "data", "models":
				collectModelList(value, out)
			}
		}
	}
}

func collectModelList(node any, out *[]string) {
	switch typed := node.(type) {
	case []any:
		for _, item := range typed {
			switch entry := item.(type) {
			case string:
				*out = append(*out, entry)
			case map[string]any:
				if name, ok := firstStringField(entry, "id", "model_name", "model"); ok {
					*out = append(*out, name)
				}
			}
		}
	case map[string]any:
		// 少数实现把模型名放在对象键上
		for key := range typed {
			*out = append(*out, key)
		}
	}
}

func firstStringField(entry map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := entry[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}

func normalizeModelNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	unique := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		unique = append(unique, name)
	}
	if len(unique) == 0 {
		return nil
	}
	sort.Strings(unique)
	return unique
}

// FetchModelOptions 依次尝试模型列表接口，返回模型名、命中的接口地址、回退提示与错误。
func FetchModelOptions(ctx context.Context, baseURL, apiKey string) ([]string, string, string, error) {
	endpoints := ModelListEndpoints(baseURL)
	if len(endpoints) == 0 {
		return nil, "", "", fmt.Errorf("Base URL 为空或格式不正确，无法获取模型列表")
	}

	client := &http.Client{Timeout: modelListTimeout}
	var firstErr error
	for index, endpoint := range endpoints {
		names, err := fetchModelNames(ctx, client, endpoint, apiKey)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(names) == 0 {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s 未返回模型", endpointPath(endpoint))
			}
			continue
		}

		warning := ""
		if index > 0 {
			warning = fmt.Sprintf("%s 不可用，已回退到 %s", endpointPath(endpoints[0]), endpointPath(endpoint))
		}
		return names, endpoint, warning, nil
	}

	if firstErr == nil {
		return nil, "", "", fmt.Errorf("未获取到模型")
	}
	if len(endpoints) > 1 {
		return nil, "", "", fmt.Errorf("%v（%s 同样不可用）", firstErr, endpointPath(endpoints[1]))
	}
	return nil, "", "", firstErr
}

func endpointPath(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Path == "" {
		return endpoint
	}
	return parsed.Path
}

// unwrapURLError 去掉 url.Error 的 `Get "URL": ` 前缀，只保留根因。
func unwrapURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

func fetchModelNames(ctx context.Context, client *http.Client, endpoint, apiKey string) ([]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if key := strings.TrimSpace(apiKey); key != "" {
		request.Header.Set("Authorization", "Bearer "+key)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", endpointPath(endpoint), unwrapURLError(err))
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s 返回 HTTP %d", endpointPath(endpoint), response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, modelListMaxBytes))
	if err != nil {
		return nil, err
	}
	return ParseModelNames(body), nil
}
