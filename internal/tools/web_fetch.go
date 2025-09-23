package tools

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	web "github.com/leonardcser/web-mcp/internal/web"
)

// WebFetchHandler returns the MCP tool handler for the "web-fetch" tool.
func WebFetchHandler(fetcher *web.Fetcher) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if ctx.Err() != nil {
			return mcp.NewToolResultError(ctx.Err().Error()), nil
		}
		url, err := req.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		ps, err := fetcher.Fetch(ctx, url)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Format the parsed content as a readable string
		content := formatPageSummary(ps)
		return mcp.NewToolResultText(content), nil
	}
}

func formatPageSummary(ps *web.PageSummary) string {
	var sb strings.Builder
	if ps.Title != "" {
		sb.WriteString("# ")
		sb.WriteString(ps.Title)
		sb.WriteString("\n\n")
	}
	if ps.Description != "" {
		sb.WriteString(ps.Description)
		sb.WriteString("\n\n")
	}
	if len(ps.Links) > 0 {
		sb.WriteString("## Links\n")
		for _, l := range ps.Links {
			sb.WriteString("- ")
			sb.WriteString(l)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString(ps.Text)
	return sb.String()
}
