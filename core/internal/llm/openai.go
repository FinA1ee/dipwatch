package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Config selects an OpenAI-compatible endpoint. DeepSeek, Qwen (DashScope compatible
// mode), Kimi and OpenAI all accept the same chat completions API.
type Config struct {
	BaseURL string
	Model   string
	APIKey  string
	// Prices in yuan per million tokens, from the provider's price list.
	PriceInPerM  float64
	PriceOutPerM float64
	// ExtraBody holds provider-specific top-level request fields, e.g. DeepSeek's
	// {"thinking": {"type": "disabled"}} to turn off reasoning tokens.
	ExtraBody map[string]any
}

// ErrNotConfigured means no API key is set, so LLM features stay off.
var ErrNotConfigured = errors.New("llm: LLM_API_KEY is not set")

// ConfigFromEnv reads LLM_* variables. Base URL and model default to DeepSeek.
func ConfigFromEnv() (Config, error) {
	c := Config{
		BaseURL: getenv("LLM_BASE_URL", "https://api.deepseek.com"),
		Model:   getenv("LLM_MODEL", "deepseek-flash"),
		APIKey:  os.Getenv("LLM_API_KEY"),
	}
	if c.APIKey == "" {
		return c, ErrNotConfigured
	}
	var err error
	if c.PriceInPerM, err = parsePrice("LLM_PRICE_IN_CNY_PER_M"); err != nil {
		return c, err
	}
	if c.PriceOutPerM, err = parsePrice("LLM_PRICE_OUT_CNY_PER_M"); err != nil {
		return c, err
	}
	if raw := os.Getenv("LLM_EXTRA_BODY"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &c.ExtraBody); err != nil {
			return c, fmt.Errorf("LLM_EXTRA_BODY must be a JSON object: %w", err)
		}
	}
	return c, nil
}

type OpenAI struct {
	client openai.Client
	cfg    Config
}

func NewOpenAI(cfg Config) *OpenAI {
	client := openai.NewClient(
		option.WithBaseURL(cfg.BaseURL),
		option.WithAPIKey(cfg.APIKey),
		option.WithRequestTimeout(60*time.Second),
		option.WithMaxRetries(2),
	)
	return &OpenAI{client: client, cfg: cfg}
}

func (o *OpenAI) Name() string { return o.cfg.Model }

func (o *OpenAI) CostMicros(tokensIn, tokensOut int64) int64 {
	return costMicros(o.cfg.PriceInPerM, o.cfg.PriceOutPerM, tokensIn, tokensOut)
}

func (o *OpenAI) Chat(ctx context.Context, messages []Message, tools []Tool) (Turn, error) {
	params := openai.ChatCompletionNewParams{
		Model:       o.cfg.Model,
		Messages:    toParams(messages),
		Temperature: openai.Float(0),
	}
	for _, t := range tools {
		params.Tools = append(params.Tools, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        t.Name,
			Description: openai.String(t.Description),
			Parameters:  openai.FunctionParameters(t.Parameters),
		}))
	}

	var opts []option.RequestOption
	for k, v := range o.cfg.ExtraBody {
		opts = append(opts, option.WithJSONSet(k, v))
	}
	res, err := o.client.Chat.Completions.New(ctx, params, opts...)
	if err != nil {
		return Turn{}, fmt.Errorf("llm chat: %w", err)
	}
	if len(res.Choices) == 0 {
		return Turn{}, errors.New("llm chat: response has no choices")
	}
	msg := res.Choices[0].Message
	turn := Turn{
		Content:   msg.Content,
		TokensIn:  res.Usage.PromptTokens,
		TokensOut: res.Usage.CompletionTokens,
	}
	for _, tc := range msg.ToolCalls {
		turn.ToolCalls = append(turn.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments})
	}
	return turn, nil
}

func toParams(messages []Message) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			out = append(out, openai.SystemMessage(m.Content))
		case RoleUser:
			out = append(out, openai.UserMessage(m.Content))
		case RoleTool:
			out = append(out, openai.ToolMessage(m.Content, m.ToolCallID))
		case RoleAssistant:
			var a openai.ChatCompletionAssistantMessageParam
			if m.Content != "" {
				a.Content.OfString = openai.String(m.Content)
			}
			for _, tc := range m.ToolCalls {
				a.ToolCalls = append(a.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: tc.Arguments,
						},
					},
				})
			}
			out = append(out, openai.ChatCompletionMessageParamUnion{OfAssistant: &a})
		}
	}
	return out
}

func costMicros(priceInPerM, priceOutPerM float64, tokensIn, tokensOut int64) int64 {
	// price per million tokens in yuan == micro-yuan per token.
	return int64(priceInPerM*float64(tokensIn) + priceOutPerM*float64(tokensOut) + 0.5)
}

func parsePrice(key string) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("%s must be a non-negative number, got %q", key, v)
	}
	return f, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
