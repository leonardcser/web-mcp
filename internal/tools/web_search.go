package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	web "github.com/leonardcser/web-mcp/internal/web"
)

// WebSearchHandler returns the MCP tool handler for the "web-search" tool.
func WebSearchHandler(searcher *web.Searcher) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		results, err := searcher.Search(ctx, q, 10)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(formatSearchResults(results)), nil
	}
}

// formatSearchResults renders an ordered list and ensures only a single URL line.
func formatSearchResults(results []web.SearchResult) string {
	if len(results) == 0 {
		return "No results."
	}
	var sb strings.Builder
	for i, r := range results {
		title := r.Title
		link := r.Link
		desc := r.Description

		sb.WriteString(fmt.Sprintf("%d. %s\n   %s", i+1, title, link))
		if desc != "" {
			sb.WriteString("\n   ")
			sb.WriteString(desc)
		}
		if i < len(results)-1 {
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}
