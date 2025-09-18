package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/leonardcser/web-mcp/internal/cache"
)

func main() {
	sock := defaultString(os.Getenv("WEB_MCP_CACHE_SOCK"), defaultSocketPath())
	db := defaultString(os.Getenv("WEB_MCP_CACHE_DB"), defaultDBPath())

	// Ensure socket dir exists and remove stale socket
	_ = os.MkdirAll(filepath.Dir(sock), 0o755)
	_ = os.Remove(sock)

	l, err := net.Listen("unix", sock)
	if err != nil {
		panic(err)
	}
	defer l.Close()
	_ = os.Chmod(sock, 0o600)

	store, err := cache.Open(db, cache.Options{Bucket: "web", DefaultTTL: 15 * time.Minute})
	if err != nil {
		panic(err)
	}
	defer store.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		go handleConn(conn, store)
	}
}

func handleConn(conn net.Conn, kv cache.KV) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var req cache.Request
		if err := dec.Decode(&req); err != nil {
			return
		}
		switch req.Op {
		case "get":
			v, err := kv.Get(req.Key)
			if err != nil {
				_ = enc.Encode(cache.Response{OK: false, Error: err.Error()})
				continue
			}
			_ = enc.Encode(cache.Response{OK: true, Value: v})
		case "put":
			ttl := time.Duration(req.TTLSeconds) * time.Second
			if err := kv.Put(req.Key, req.Value, ttl); err != nil {
				_ = enc.Encode(cache.Response{OK: false, Error: err.Error()})
				continue
			}
			_ = enc.Encode(cache.Response{OK: true})
		case "delete":
			if err := kv.Delete(req.Key); err != nil {
				_ = enc.Encode(cache.Response{OK: false, Error: err.Error()})
				continue
			}
			_ = enc.Encode(cache.Response{OK: true})
		default:
			_ = enc.Encode(cache.Response{OK: false, Error: "unknown op"})
		}
	}
}

func defaultSocketPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".cache", "web-mcp", "cache.sock")
}

func defaultDBPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".cache", "web-mcp", "cache.bbolt")
}

func defaultString(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
