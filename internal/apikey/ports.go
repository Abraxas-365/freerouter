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

// Store abstracts the IAMKit service-account backend.
// There is no local database — IAMKit owns the storage.
type Store interface {
	Create(ctx context.Context, input CreateServiceAccount, applicationID, resourceID string) (ServiceAccountCredential, error)
	List(ctx context.Context) ([]ServiceAccount, error)
	ListApplications(ctx context.Context) ([]Application, error)
	Revoke(ctx context.Context, id string) error
}
