/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package interfaces

import (
	"fmt"
	"time"
)

// AuthMethod specifies the authentication method for tunnel connections
type AuthMethod string

const (
	// AuthMethodNone indicates no authentication (for testing only)
	AuthMethodNone AuthMethod = "none"
	
	// AuthMethodToken uses bearer token authentication
	AuthMethodToken AuthMethod = "token"
	
	// AuthMethodCertificate uses client certificate authentication
	AuthMethodCertificate AuthMethod = "certificate"
	
	// AuthMethodMTLS uses mutual TLS authentication
	AuthMethodMTLS AuthMethod = "mtls"
	
	// AuthMethodOAuth2 uses OAuth2 authentication flow
	AuthMethodOAuth2 AuthMethod = "oauth2"
	
	// AuthMethodServiceAccount uses Kubernetes service account tokens
	AuthMethodServiceAccount AuthMethod = "serviceaccount"
)

// OAuth2Config contains OAuth2 authentication parameters
type OAuth2Config struct {
	// ClientID identifies the OAuth2 client
	ClientID string
	
	// ClientSecret authenticates the OAuth2 client
	ClientSecret string
	
	// TokenURL is the OAuth2 token endpoint
	TokenURL string
	
	// Scopes contains requested OAuth2 scopes
	Scopes []string
	
	// RefreshToken for token renewal
	RefreshToken string
	
	// AccessToken contains current access token
	AccessToken string
	
	// TokenExpiry tracks when current token expires
	TokenExpiry time.Time
}

// ServiceAccountConfig contains Kubernetes service account authentication parameters
type ServiceAccountConfig struct {
	// TokenFile contains path to service account token file
	TokenFile string
	
	// Namespace specifies the service account namespace
	Namespace string
	
	// ServiceAccount specifies the service account name
	ServiceAccount string
	
	// Audience specifies the intended audience for the token
	Audience string
	
	// TokenExpiration sets custom token expiration duration
	TokenExpiration time.Duration
}

// AuthCredentials contains authentication credentials for tunnel connections
type AuthCredentials struct {
	// Method specifies which authentication method to use
	Method AuthMethod
	
	// Token contains bearer token for token-based authentication
	Token string
	
	// TokenFile contains path to file containing bearer token
	TokenFile string
	
	// CertificateData contains PEM-encoded client certificate
	CertificateData []byte
	
	// CertificateFile contains path to client certificate file
	CertificateFile string
	
	// KeyData contains PEM-encoded private key
	KeyData []byte
	
	// KeyFile contains path to private key file
	KeyFile string
	
	// CAData contains PEM-encoded certificate authority certificates
	CAData []byte
	
	// CAFile contains path to certificate authority file
	CAFile string
	
	// ServerName for certificate validation (overrides default)
	ServerName string
	
	// InsecureSkipTLSVerify skips server certificate verification (dangerous)
	InsecureSkipTLSVerify bool
	
	// OAuth2Config contains OAuth2 authentication configuration
	OAuth2Config *OAuth2Config
	
	// ServiceAccountConfig contains service account authentication configuration
	ServiceAccountConfig *ServiceAccountConfig
}

// Common authentication errors that are used across multiple files
var (
	// ErrUnsupportedAuthMethod indicates auth method is not supported
	ErrUnsupportedAuthMethod = fmt.Errorf("unsupported authentication method")
	
	// ErrInvalidCredentials indicates malformed or incomplete credentials
	ErrInvalidCredentials = fmt.Errorf("invalid credentials")
)