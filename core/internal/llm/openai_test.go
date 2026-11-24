package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A minimal stand-in for an OpenAI-compatible /chat/completions endpoint.
func fakeProvider(t *testing.T, gotBody *map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path: got %s, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization header: got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "id": "x", "object": "chat.completion", "created": 1, "model": "deepseek-chat",
		  "choices": [{"index": 0, "finish_reason": "tool_calls", "message": {
		    "role": "assistant", "content": "",
		    "tool_calls": [{"id": "call_1", "type": "function",
		      "function": {"name": "browser_navigate", "arguments": "{\"url\":\"https://shop.test/s?q=x\"}"}}]
		  }}],
		  "usage": {"prompt_tokens": 1200, "completion_tokens": 80, "total_tokens": 1280}
		}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestChatSendsToolsAndParsesToolCalls(t *testing.T) {
	var body map[string]any
	srv := fakeProvider(t, &body)
	m := NewOpenAI(Config{BaseURL: srv.URL, Model: "deepseek-flash", APIKey: "test-key", PriceInPerM: 2, PriceOutPerM: 8,
		ExtraBody: map[string]any{"thinking": map[string]any{"type": "disabled"}}})

	turn, err := m.Chat(context.Background(), []Message{
		{Role: RoleSystem, Content: "rules"},
		{Role: RoleUser, Content: "go"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call_0", Name: "browser_snapshot", Arguments: "{}"}}},
		{Role: RoleTool, ToolCallID: "call_0", Content: "page text"},
	}, []Tool{{Name: "browser_navigate", Description: "open a URL", Parameters: map[string]any{"type": "object"}}})
	if err != nil {
		t.Fatal(err)
	}

	if len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != "browser_navigate" || turn.ToolCalls[0].ID != "call_1" {
		t.Fatalf("tool calls: %+v", turn.ToolCalls)
	}
	if turn.TokensIn != 1200 || turn.TokensOut != 80 {
		t.Fatalf("usage: in=%d out=%d", turn.TokensIn, turn.TokensOut)
	}

	msgs, _ := body["messages"].([]any)
	if len(msgs) != 4 {
		t.Fatalf("sent %d messages, want 4", len(msgs))
	}
	tool, _ := msgs[3].(map[string]any)
	if tool["role"] != "tool" || tool["tool_call_id"] != "call_0" {
		t.Fatalf("tool message: %v", tool)
	}
	tools, _ := body["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("sent %d tools, want 1", len(tools))
	}
	if body["model"] != "deepseek-flash" {
		t.Fatalf("model: %v", body["model"])
	}
	thinking, _ := body["thinking"].(map[string]any)
	if thinking["type"] != "disabled" {
		t.Fatalf("extra body not sent: thinking=%v", body["thinking"])
	}
}

func TestCostMicros(t *testing.T) {
	m := NewOpenAI(Config{PriceInPerM: 2, PriceOutPerM: 8})
	// 1200 * 2 + 80 * 8 = 3040 micro-yuan = 0.00304 yuan
	if got := m.CostMicros(1200, 80); got != 3040 {
		t.Fatalf("cost: got %d, want 3040", got)
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	if _, err := ConfigFromEnv(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("no key: got %v, want ErrNotConfigured", err)
	}

	t.Setenv("LLM_API_KEY", "k")
	t.Setenv("LLM_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
	t.Setenv("LLM_MODEL", "qwen-plus")
	t.Setenv("LLM_PRICE_IN_CNY_PER_M", "0.8")
	t.Setenv("LLM_PRICE_OUT_CNY_PER_M", "abc")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("bad price: want error")
	}

	t.Setenv("LLM_PRICE_OUT_CNY_PER_M", "2")
	t.Setenv("LLM_EXTRA_BODY", `{"thinking":{"type":"disabled"}}`)
	c, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if c.Model != "qwen-plus" || c.PriceInPerM != 0.8 || c.PriceOutPerM != 2 || c.ExtraBody["thinking"] == nil {
		t.Fatalf("config: %+v", c)
	}
}
