package agent

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type navigateArgs struct {
	URL string `json:"url" jsonschema:"the URL to open"`
}

// TestMCPBrowserAgainstInMemoryServer exercises the real MCP client code against a
// small MCP server that mimics two Playwright tools.
func TestMCPBrowserAgainstInMemoryServer(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "fake-playwright", Version: "0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "browser_navigate", Description: "Navigate to a URL"},
		func(_ context.Context, _ *mcp.CallToolRequest, in navigateArgs) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "opened " + in.URL}}}, nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "browser_click", Description: "Click"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "element not found"}}}, nil, nil
		})

	serverT, clientT := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = serverSession.Close() }()

	b, err := Connect(ctx, clientT)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	tools, err := b.Tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(tools))
	}
	for _, tool := range tools {
		if tool.Name == "browser_navigate" {
			props, _ := tool.Parameters["properties"].(map[string]any)
			if _, ok := props["url"]; !ok {
				t.Fatalf("navigate schema lacks url: %v", tool.Parameters)
			}
		}
	}

	out, err := b.Call(ctx, "browser_navigate", map[string]any{"url": "https://www.newegg.com/"})
	if err != nil || out != "opened https://www.newegg.com/" {
		t.Fatalf("navigate: %q, %v", out, err)
	}
	if _, err := b.Call(ctx, "browser_click", map[string]any{}); err == nil {
		t.Fatal("tool error should surface as an error")
	}
}
