package cache

import (
	"encoding/json"
	"net"
	"time"
)

// Client implements KV over a Unix socket.
type Client struct {
	socketPath string
}

func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

func (c *Client) withConn(fn func(conn net.Conn) error) error {
	conn, err := net.DialTimeout("unix", c.socketPath, 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	return fn(conn)
}

func (c *Client) Get(key string) ([]byte, error) {
	var out []byte
	err := c.withConn(func(conn net.Conn) error {
		enc := json.NewEncoder(conn)
		dec := json.NewDecoder(conn)
		req := Request{Op: "get", Key: key}
		if err := enc.Encode(&req); err != nil {
			return err
		}
		var resp Response
		if err := dec.Decode(&resp); err != nil {
			return err
		}
		if !resp.OK {
			if resp.Error == "cache: not found" {
				return ErrNotFound
			}
			if resp.Error == "cache: expired" {
				return ErrExpired
			}
			return errorsNew(resp.Error)
		}
		out = append([]byte(nil), resp.Value...)
		return nil
	})
	return out, err
}

func (c *Client) Put(key string, value []byte, ttl time.Duration) error {
	return c.withConn(func(conn net.Conn) error {
		enc := json.NewEncoder(conn)
		dec := json.NewDecoder(conn)
		req := Request{Op: "put", Key: key, Value: value, TTLSeconds: int64(ttl / time.Second)}
		if err := enc.Encode(&req); err != nil {
			return err
		}
		var resp Response
		if err := dec.Decode(&resp); err != nil {
			return err
		}
		if !resp.OK {
			return errorsNew(resp.Error)
		}
		return nil
	})
}

func (c *Client) Delete(key string) error {
	return c.withConn(func(conn net.Conn) error {
		enc := json.NewEncoder(conn)
		dec := json.NewDecoder(conn)
		req := Request{Op: "delete", Key: key}
		if err := enc.Encode(&req); err != nil {
			return err
		}
		var resp Response
		if err := dec.Decode(&resp); err != nil {
			return err
		}
		if !resp.OK {
			return errorsNew(resp.Error)
		}
		return nil
	})
}

// Local helper to avoid importing fmt just for errors.
func errorsNew(msg string) error { return &simpleError{s: msg} }

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }
