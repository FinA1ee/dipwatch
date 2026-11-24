package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/FinA1ee/dipwatch/core/internal/llm"
)

// Browser is what the agent needs from a browser: its tools, and a way to call them.
type Browser interface {
	Tools(ctx context.Context) ([]llm.Tool, error)
	Call(ctx context.Context, name string, args map[string]any) (string, error)
	Close() error
}

// MCPBrowser drives a browser through an MCP server, normally Playwright MCP.
type MCPBrowser struct {
	session *mcp.ClientSession
	tempDir string // Playwright's output directory, removed on Close
}

// PlaywrightArgs are the flags every run uses: no window, no saved profile, no
// screenshots in tool results (they cost tokens and our model reads text).
var PlaywrightArgs = []string{
	"--headless",
	"--isolated",
	"--browser", "chromium",
	"--image-responses", "omit",
}

// StartPlaywright launches Playwright MCP as a child process. command is the
// executable plus any leading arguments, for example ["npx", "-y", "@playwright/mcp@0.0.86"].
func StartPlaywright(ctx context.Context, command []string, extraArgs ...string) (*MCPBrowser, error) {
	if len(command) == 0 {
		return nil, errors.New("browser: empty command")
	}
	// Playwright MCP writes every snapshot to a file in its output directory. Give
	// each browser its own temporary directory so files never pile up.
	dir, err := os.MkdirTemp("", "dipwatch-browser-")
	if err != nil {
		return nil, fmt.Errorf("browser: temp dir: %w", err)
	}
	args := append(append(append([]string{}, command[1:]...), PlaywrightArgs...), extraArgs...)
	args = append(args, "--output-dir", dir)
	cmd := exec.Command(command[0], args...) //nolint:gosec // command comes from server config, not users
	cmd.Dir = dir
	b, err := Connect(ctx, &mcp.CommandTransport{Command: cmd})
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	b.tempDir = dir
	return b, nil
}

// Connect opens an MCP session over any transport. Tests use an in-memory transport.
func Connect(ctx context.Context, transport mcp.Transport) (*MCPBrowser, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "dipwatch-agent", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("browser: connect: %w", err)
	}
	return &MCPBrowser{session: session}, nil
}

func (b *MCPBrowser) Tools(ctx context.Context) ([]llm.Tool, error) {
	res, err := b.session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("browser: list tools: %w", err)
	}
	tools := make([]llm.Tool, 0, len(res.Tools))
	for _, t := range res.Tools {
		schema, _ := t.InputSchema.(map[string]any)
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		tools = append(tools, llm.Tool{Name: t.Name, Description: t.Description, Parameters: schema})
	}
	return tools, nil
}

// Call runs a tool and returns its text output. A tool that reports an error
// returns that text with an error, so the agent can show it to the model.
func (b *MCPBrowser) Call(ctx context.Context, name string, args map[string]any) (string, error) {
	res, err := b.session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", fmt.Errorf("browser: %s: %w", name, err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			if text.Len() > 0 {
				text.WriteString("\n")
			}
			text.WriteString(t.Text)
		}
	}
	if res.IsError {
		return text.String(), fmt.Errorf("browser: %s failed: %s", name, truncate(text.String(), 300))
	}
	return text.String(), nil
}

func (b *MCPBrowser) Close() error {
	err := b.session.Close()
	if b.tempDir != "" {
		_ = os.RemoveAll(b.tempDir)
	}
	return err
}
