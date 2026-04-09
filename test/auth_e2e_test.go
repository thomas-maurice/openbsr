// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

package test

import (
	"testing"
)

// TestAuthFlow verifies the core authentication lifecycle:
// register → login → access protected endpoint → reject unauthenticated.
func TestAuthFlow(t *testing.T) {
	register(t, "authuser", "password123")
	token := login(t, "authuser", "password123")

	// Authenticated user can fetch their own profile
	me := apiRequest(t, "GET", "/api/v1/auth/me", nil, token)
	if me["username"] != "authuser" {
		t.Fatalf("expected username authuser, got: %v", me)
	}

	// Unauthenticated request to /me is rejected with 401
	noAuth := apiRequest(t, "GET", "/api/v1/auth/me", nil, "")
	assertStatus(t, noAuth, 401, "unauthenticated /me")
}

// TestDuplicateRegistration verifies that registering an existing username
// returns 409 Conflict instead of silently creating a duplicate.
func TestDuplicateRegistration(t *testing.T) {
	register(t, "dupuser", "password123")
	r := apiRequest(t, "POST", "/api/v1/auth/register", map[string]string{
		"username": "dupuser", "password": "password123",
	}, "")
	assertStatus(t, r, 409, "duplicate registration")
}

// TestTokenManagement verifies the API token lifecycle:
// create a named token, list tokens (login tokens filtered out), revoke by ID.
func TestTokenManagement(t *testing.T) {
	register(t, "tokenuser", "password123")
	token := login(t, "tokenuser", "password123")

	// Create a named API token
	r := apiRequest(t, "POST", "/api/v1/auth/tokens",
		map[string]string{"note": "my-api-token"}, token)
	apiToken, _ := r["token"].(string)
	tokenID, _ := r["id"].(string)
	if apiToken == "" || tokenID == "" {
		t.Fatal("create token: missing token or id")
	}

	// List tokens — should include the API token but NOT login session tokens
	tokens := apiRequestArray(t, "GET", "/api/v1/auth/tokens", token)
	found := false
	for _, tok := range tokens {
		if tok["note"] == "my-api-token" {
			found = true
		}
		if tok["note"] == "login" {
			t.Fatal("login session tokens should be filtered from the list")
		}
	}
	if !found {
		t.Fatal("created API token not found in token list")
	}

	// Revoke the token by ID
	r = apiRequest(t, "DELETE", "/api/v1/auth/tokens/"+tokenID, nil, token)
	if r["status"] != "revoked" {
		t.Fatalf("revoke token: %v", r)
	}
}
