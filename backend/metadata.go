package backend

// Metadata carries protocol-neutral request context. Maps are detached copies
// and must be treated as immutable by backend implementations.
type Metadata struct {
	RequestID  string
	CallerID   string
	Protocol   string
	Extensions map[string][]string
}
