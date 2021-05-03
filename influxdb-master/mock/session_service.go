package mock

import (
	"context"
	"fmt"
	"time"

	platform "github.com/influxdata/influxdb/v2"
)

// SessionService is a mock implementation of a retention.SessionService, which
// also makes it a suitable mock to use wherever an platform.SessionService is required.
type SessionService struct {
	FindSessionFn   func(context.Context, string) (*platform.Session, error)
	ExpireSessionFn func(context.Context, string) error
	CreateSessionFn func(context.Context, string) (*platform.Session, error)
	RenewSessionFn  func(ctx context.Context, session *platform.Session, newExpiration time.Time) error
}

// NewSessionService returns a mock SessionService where its methods will return
// zero values.
func NewSessionService() *SessionService {
	return &SessionService{
		FindSessionFn:   func(context.Context, string) (*platform.Session, error) { return nil, fmt.Errorf("mock session") },
		CreateSessionFn: func(context.Context, string) (*platform.Session, error) { return nil, fmt.Errorf("mock session") },
		ExpireSessionFn: func(context.Context, string) error { return fmt.Errorf("mock session") },
		RenewSessionFn: func(ctx context.Context, session *platform.Session, expiredAt time.Time) error {
			return fmt.Errorf("mock session")
		},
	}
}

// FindSession returns the session found at the provided key.
func (s *SessionService) FindSession(ctx context.Context, key string) (*platform.Session, error) {
	return s.FindSessionFn(ctx, key)
}

// CreateSession creates a session for a user with the users maximal privileges.
func (s *SessionService) CreateSession(ctx context.Context, user string) (*platform.Session, error) {
	return s.CreateSessionFn(ctx, user)
}

// ExpireSession expires the session provided at key.
func (s *SessionService) ExpireSession(ctx context.Context, key string) error {
	return s.ExpireSessionFn(ctx, key)
}

// RenewSession extends the expire time to newExpiration.
func (s *SessionService) RenewSession(ctx context.Context, session *platform.Session, expiredAt time.Time) error {
	return s.RenewSessionFn(ctx, session, expiredAt)
}
