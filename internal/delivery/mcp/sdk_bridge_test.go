package mcp

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	cortexServer "github.com/FreePeak/cortex/pkg/server"
	cortexTools "github.com/FreePeak/cortex/pkg/tools"
	sdkAuth "github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/FreePeak/db-mcp-server/internal/usecase"
)

func TestServerWrapperSDKStreamableRoundTrip(t *testing.T) {
	wrapper := NewServerWrapper(cortexServer.NewMCPServer("test", "1.0.0", nil))
	tool := cortexTools.NewTool(
		"echo",
		cortexTools.WithDescription("echo text"),
		cortexTools.WithString("text", cortexTools.Description("text to echo"), cortexTools.Required()),
	)
	var seen usecase.Caller
	err := wrapper.AddTool(context.Background(), tool, func(ctx context.Context, request cortexServer.ToolCallRequest) (interface{}, error) {
		seen = usecase.CallerFromContext(ctx)
		text := request.Parameters["text"].(string)
		if text == "error" {
			return nil, errors.New("tool failed")
		}
		return &Response{
			Content:  []TextContent{{Type: "text", Text: "echo:" + text}},
			Metadata: map[string]interface{}{"transactionId": "tx-test"},
		}, nil
	})
	require.NoError(t, err)

	httpServer := httptest.NewServer(NewStreamableHTTPHandler(wrapper.SDKServer()))
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.StreamableClientTransport{
		Endpoint:             httpServer.URL,
		HTTPClient:           httpServer.Client(),
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tools.Tools, 1)
	require.Equal(t, "echo", tools.Tools[0].Name)
	schema, ok := tools.Tools[0].InputSchema.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "object", schema["type"])
	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, properties, "text")

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"text": "hello"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	require.Equal(t, "echo:hello", textContent.Text)
	require.Equal(t, "tx-test", result.Meta["transactionId"])
	require.NotEmpty(t, seen.Session, "streamable tool call must carry the MCP session ID to the use case")

	errorResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"text": "error"},
	})
	require.NoError(t, err)
	require.True(t, errorResult.IsError)
	require.Len(t, errorResult.Content, 1)
	errorText, ok := errorResult.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	require.Equal(t, "tool failed", errorText.Text)

	_, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "echo",
		Arguments: []string{"not-an-object"},
	})
	require.Error(t, err)
}

func TestToolScopeAllowed(t *testing.T) {
	request := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: "query"},
		Extra:  &sdkmcp.RequestExtra{TokenInfo: &sdkAuth.TokenInfo{Scopes: []string{"db:read"}}},
	}
	require.True(t, toolScopeAllowed(request))
	request.Params.Name = "execute"
	require.False(t, toolScopeAllowed(request))
	request.Params.Name = "query"
	request.Extra.TokenInfo.Scopes = []string{"db:admin"}
	request.Params.Name = "execute"
	require.True(t, toolScopeAllowed(request))
}

func TestAdaptCortexHandlerPassesPrincipalToUseCase(t *testing.T) {
	var seen usecase.Caller
	handler := adaptCortexHandler(func(ctx context.Context, _ cortexServer.ToolCallRequest) (interface{}, error) {
		seen = usecase.CallerFromContext(ctx)
		return nil, nil
	})

	authed := &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: "query"},
		Extra:  &sdkmcp.RequestExtra{TokenInfo: &sdkAuth.TokenInfo{UserID: "user@example.com", Scopes: []string{"db:read"}}},
	}
	_, err := handler(context.Background(), authed)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", seen.Principal)

	unauthenticated := &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{Name: "query"}}
	_, err = handler(context.Background(), unauthenticated)
	require.NoError(t, err)
	require.Empty(t, seen.Principal)
}
