package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/leonardcser/web-mcp/internal/cache"
	"github.com/leonardcser/web-mcp/internal/logger"
	tools "github.com/leonardcser/web-mcp/internal/tools"
	web "github.com/leonardcser/web-mcp/internal/web"
)

func main() {
	if err := logger.InitFromEnv(); err != nil {
		panic(err)
	}
	defer logger.Close()

	logger.Infof("Starting Web MCP server")

	// Connect to cache daemon; start it if needed, then connect.
	sock := defaultSocketPath()
	logger.Infof("Attempting to connect to cache daemon at %s", sock)
	client, err := connectCache(sock)
	if err != nil {
		logger.Warnf("Failed to connect to cache daemon: %v, attempting to start daemon", err)
		// attempt to start daemon
		if startErr := startCacheDaemon(); startErr != nil {
			logger.Errorf("Failed to start cache daemon: %v", startErr)
		} else {
			logger.Infof("Cache daemon started successfully")
		}
		// wait for socket to appear
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if c2, err2 := connectCache(sock); err2 == nil {
				client = c2
				err = nil
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if client == nil {
			logger.Errorf("Failed to connect to cache daemon after startup attempt: %v", err)
			panic(err)
		}
	}
	logger.Infof("Successfully connected to cache daemon")

	fetcher := web.NewFetcher(client, 15*time.Minute)
	searcher := web.NewSearcher(client, 5*time.Minute)
	logger.Infof("Initialized web fetcher and searcher with cache client")

	s := server.NewMCPServer(
		"Web MCP",
		"0.1.0",
		server.WithRecovery(),
		server.WithToolCapabilities(false),
	)
	logger.Infof("Created MCP server instance")

	toolFetch := mcp.NewTool("web-fetch",
		mcp.WithDescription(multiline(
			"Fetches content from a specified URL and returns the parsed content",
			"\nFunctionality:",
			"- Takes a URL as input",
			"- Fetches the URL content and parses it",
			"- Returns the structured content including title, description, text, and links",
			"\nUsage notes:",
			"- If an MCP-provided web fetch tool is available, prefer using that tool instead",
			"- The URL must be a fully-formed valid URL",
			"- HTTP URLs will be automatically upgraded to HTTPS",
			"- This tool is read-only and does not modify any files",
			"- Includes a self-cleaning 15-minute cache for faster responses when repeatedly accessing the same URL",
			"- When a URL redirects to a different host, the tool will inform you and provide the redirect URL in a special format",
		)),
		mcp.WithString("url", mcp.Required(), mcp.Description("The URL to fetch content from")),
	)
	s.AddTool(toolFetch, tools.WebFetchHandler(fetcher))
	logger.Infof("Registered web-fetch tool")

	toolSearch := mcp.NewTool("web-search",
		mcp.WithDescription(multiline(
			"Allows you to search the web and use the results to inform responses",
			"\nFunctionality:",
			"- Provides up-to-date information for current events and recent data",
			"- Returns search result information formatted as search result blocks",
			"- Use this tool for accessing information beyond your knowledge cutoff",
			"- Searches are performed automatically within a single API call",
			"\nUsage notes:",
			"- Web search is only available in the US",
			"- Account for Today's date in environment (e.g., use 2025 when appropriate)",
		)),
		mcp.WithString("query", mcp.Required(), mcp.Description("The search query to use")),
	)
	s.AddTool(toolSearch, tools.WebSearchHandler(searcher))
	logger.Infof("Registered web-search tool")

	logger.Infof("Starting MCP server on stdio")
	if err := server.ServeStdio(s); err != nil {
		logger.Errorf("server error: %v", err)
	}
}

// multiline joins lines with newlines for tool descriptions.
func multiline(lines ...string) string { return strings.Join(lines, "\n") }

func defaultSocketPath() string {
	if s := os.Getenv("WEB_MCP_CACHE_SOCK"); s != "" {
		return s
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".cache", "web-mcp", "cache.sock")
}

func connectCache(sock string) (cache.KV, error) {
	// quick probe
	conn, err := net.DialTimeout("unix", sock, 200*time.Millisecond)
	if err != nil {
		return nil, err
	}
	_ = conn.Close()
	return cache.NewClient(sock), nil
}

func startCacheDaemon() error {
	// 1) Try cache binary next to this server executable (works with absolute invocation)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		sibling := filepath.Join(exeDir, "web-mcp-cache")
		if _, statErr := os.Stat(sibling); statErr == nil {
			cmd := exec.Command(sibling)
			cmd.Stdout = nil
			cmd.Stderr = nil
			cmd.Env = os.Environ()
			return cmd.Start()
		}
	}

	// 2) Try PATH binary
	if path, err := exec.LookPath("web-mcp-cache"); err == nil {
		cmd := exec.Command(path)
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Env = os.Environ()
		return cmd.Start()
	}

	// 3) Try local binary in current working directory (best-effort)
	if _, err := os.Stat("./web-mcp-cache"); err == nil {
		cmd := exec.Command("./web-mcp-cache")
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Env = os.Environ()
		return cmd.Start()
	}

	return exec.ErrNotFound
}
