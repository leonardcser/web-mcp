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
		url, err := req.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		ps, err := fetcher.Fetch(url)
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
	sb.WriteString("Page Content:\n")
	sb.WriteString("=============\n\n")
	sb.WriteString("Title: ")
	sb.WriteString(ps.Title)
	sb.WriteString("\n\nDescription: ")
	sb.WriteString(ps.Description)
	sb.WriteString("\n\nURL: ")
	sb.WriteString(ps.URL)
	sb.WriteString("\n\nLinks found on the page:\n")
	for _, l := range ps.Links {
		sb.WriteString("- ")
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	sb.WriteString("\nFull Text Content:\n")
	sb.WriteString("==================\n")
	sb.WriteString(ps.Text)
	return sb.String()
}
