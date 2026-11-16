package apikey

import "context"

// Commands defines write operations for service accounts.
type Commands interface {
	Create(ctx context.Context, input CreateServiceAccount) (ServiceAccountCredential, error)
	Revoke(ctx context.Context, id string) error
}

// Queries defines read operations for service accounts.
type Queries interface {
	List(ctx context.Context) ([]ServiceAccount, error)
	ListApplications(ctx context.Context) ([]Application, error)
}

// Store abstracts the service-account backend: IAMKit (iam mode) or
// FreeRouter's own gateway keys (none/token modes).
type Store interface {
	Create(ctx context.Context, input CreateServiceAccount, applicationID, resourceID string) (ServiceAccountCredential, error)
	List(ctx context.Context) ([]ServiceAccount, error)
	Find(ctx context.Context, id string) (ServiceAccount, error)
	ListApplications(ctx context.Context) ([]Application, error)
	Revoke(ctx context.Context, id string) error
}

// LocalStore is the Store of keys FreeRouter issues itself: it also
// resolves a presented secret to its live key (not revoked, not expired);
// anything else is errx.Unauthorized.
type LocalStore interface {
	Store
	Resolve(ctx context.Context, secret string) (ServiceAccount, error)
}
