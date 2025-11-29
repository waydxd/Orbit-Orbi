package orbi

import "context"

// Agent is the interface for the Orbi agent.
type Agent interface {
	Chat(ctx context.Context, message string) (string, error)
	Close() error
}
