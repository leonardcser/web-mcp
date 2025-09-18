package cache

// Simple JSON protocol for cache daemon over a Unix domain socket.
// One request -> one response using json.Encoder/Decoder per connection.

type Request struct {
	Op         string `json:"op"` // "get" | "put" | "delete"
	Key        string `json:"key"`
	Value      []byte `json:"value,omitempty"`
	TTLSeconds int64  `json:"ttl_seconds,omitempty"`
}

type Response struct {
	OK    bool   `json:"ok"`
	Value []byte `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
}
