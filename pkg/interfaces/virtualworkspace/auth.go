package virtualworkspace

import (
	"context"
	"crypto/x509"
	"net/http"
	"time"

	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// Authenticator authenticates requests to virtual workspace
type Authenticator interface {
	// AuthenticateRequest validates request authentication
	AuthenticateRequest(req *http.Request) (*user.DefaultInfo, bool, error)

	// GetUser extracts user from context
	GetUser(ctx context.Context) (user.Info, error)
}

// Authorizer authorizes requests to virtual workspace
type Authorizer interface {
	// Authorize checks if request is authorized
	Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error)
}

// TokenValidator validates bearer tokens
type TokenValidator interface {
	// ValidateToken checks token validity
	ValidateToken(ctx context.Context, token string) (*user.DefaultInfo, error)

	// RefreshToken refreshes an expired token
	RefreshToken(ctx context.Context, token string) (string, error)
}

// CertificateValidator validates client certificates
type CertificateValidator interface {
	// ValidateCertificate checks certificate validity
	ValidateCertificate(cert *x509.Certificate) (*user.DefaultInfo, error)
}

// ImpersonationHandler handles impersonation
type ImpersonationHandler interface {
	// HandleImpersonation processes impersonation headers
	HandleImpersonation(req *http.Request, user user.Info) (user.Info, error)

	// CanImpersonate checks if user can impersonate
	CanImpersonate(user user.Info, targetUser string) bool
}

// AuditLogger logs API requests
type AuditLogger interface {
	// LogRequest logs an API request
	LogRequest(
		ctx context.Context,
		user user.Info,
		req *http.Request,
		response *http.Response,
		duration time.Duration,
	) error
}