package localwriter

// selectBackend returns the storage backend. Only JSONL is supported.
func selectBackend(_ string) Backend {
	return &JSONLBackend{}
}
