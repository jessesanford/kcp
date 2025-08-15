/*
Copyright 2025 The KCP Authors.

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

package auth

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
)

func TestBasicProvider_ValidateToken(t *testing.T) {
	tests := map[string]struct {
		token     string
		wantError bool
		wantUser  string
	}{
		"empty token": {
			token:     "",
			wantError: true,
		},
		"bearer token": {
			token:     "Bearer test-token",
			wantError: false,
			wantUser:  "user-test-tok",
		},
		"basic auth token": {
			token:     base64.StdEncoding.EncodeToString([]byte("testuser:testpass")),
			wantError: false,
			wantUser:  "testuser",
		},
		"opaque token": {
			token:     "opaque-token-value",
			wantError: false,
			wantUser:  "user-opaque-t",
		},
	}

	provider := NewBasicProvider(DefaultConfig())
	ctx := context.Background()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tokenInfo, err := provider.ValidateToken(ctx, tc.token)

			if tc.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tokenInfo == nil {
				t.Error("expected token info, got nil")
				return
			}

			if tokenInfo.Subject.User.GetName() != tc.wantUser {
				t.Errorf("expected user %s, got %s", tc.wantUser, tokenInfo.Subject.User.GetName())
			}

			if len(tokenInfo.Subject.Groups) == 0 {
				t.Error("expected groups, got none")
			}
		})
	}
}

func TestBasicProvider_ExtractSubject(t *testing.T) {
	provider := NewBasicProvider(DefaultConfig())

	t.Run("no user in context", func(t *testing.T) {
		ctx := context.Background()
		_, err := provider.ExtractSubject(ctx)
		if err == nil {
			t.Error("expected error for missing user")
		}
	})

	t.Run("user in context", func(t *testing.T) {
		userInfo := &user.DefaultInfo{
			Name:   "testuser",
			Groups: []string{"system:authenticated"},
		}
		ctx := genericapirequest.WithUser(context.Background(), userInfo)

		subject, err := provider.ExtractSubject(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if subject == nil {
			t.Error("expected subject, got nil")
			return
		}

		if subject.User.GetName() != "testuser" {
			t.Errorf("expected user testuser, got %s", subject.User.GetName())
		}
	})
}

func TestBasicProvider_IsReady(t *testing.T) {
	provider := NewBasicProvider(DefaultConfig())
	err := provider.IsReady()
	if err != nil {
		t.Errorf("expected provider to be ready, got error: %v", err)
	}
}

func TestBasicProvider_TokenCaching(t *testing.T) {
	config := DefaultConfig()
	config.CacheTTL = 100 * time.Millisecond
	provider := NewBasicProvider(config).(*BasicProvider)

	ctx := context.Background()
	token := "test-token"

	// First validation
	tokenInfo1, err := provider.ValidateToken(ctx, token)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Second validation should use cache
	tokenInfo2, err := provider.ValidateToken(ctx, token)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if tokenInfo1.Subject.User.GetName() != tokenInfo2.Subject.User.GetName() {
		t.Error("cached result should match original")
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third validation should not use expired cache
	tokenInfo3, err := provider.ValidateToken(ctx, token)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if tokenInfo3 == nil {
		t.Error("expected valid token info after cache expiry")
	}
}

func TestExtractClusterFromPath(t *testing.T) {
	tests := map[string]struct {
		path     string
		expected string
	}{
		"no cluster in path": {
			path:     "/api/v1/pods",
			expected: "",
		},
		"cluster in path": {
			path:     "/clusters/test-cluster/api/v1/pods",
			expected: "test-cluster",
		},
		"nested cluster path": {
			path:     "/services/workspace/clusters/my-workspace/api/v1/services",
			expected: "my-workspace",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := extractClusterFromPath(tc.path)
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}