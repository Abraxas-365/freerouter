package auth

import (
	"context"
	"github.com/Abraxas-365/freerouter/internal/iam/tenant"
	"github.com/Abraxas-365/freerouter/internal/iam/user"
)

// InvitationAcceptor atomically applies verified membership, grants and acceptance.
// The caller must first prove ownership of candidate.Email.
type InvitationAcceptor interface {
	Accept(context.Context, string, user.User) (*user.User, *tenant.Tenant, error)
}
