// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

package test

import "testing"

// TestConnectRPCServices is a smoke test for the ConnectRPC service endpoints
// that the buf CLI depends on: AuthnService, UserService, OwnerService.
// These are the endpoints hit by `buf registry login` and module resolution.
func TestConnectRPCServices(t *testing.T) {
	token := login(t, "admin", "changeme")

	// AuthnService.GetCurrentUser (buf.alpha.registry.v1alpha1)
	// This is the endpoint buf CLI calls to validate credentials.
	t.Run("GetCurrentUser", func(t *testing.T) {
		r := connectRequest(t, "buf.alpha.registry.v1alpha1.AuthnService/GetCurrentUser", map[string]any{}, token)
		user, ok := r["user"].(map[string]any)
		if !ok || user["username"] != "admin" {
			t.Fatalf("expected admin user, got: %v", r)
		}
	})

	// UserService.GetUsers (buf.registry.owner.v1)
	// Used by buf CLI to resolve user references.
	t.Run("GetUsers", func(t *testing.T) {
		r := connectRequest(t, "buf.registry.owner.v1.UserService/GetUsers", map[string]any{
			"userRefs": []map[string]string{{"name": "admin"}},
		}, token)
		users, ok := r["users"].([]any)
		if !ok || len(users) == 0 {
			t.Fatalf("expected users, got: %v", r)
		}
	})

	// OwnerService.GetOwners (buf.registry.owner.v1)
	// Used by buf CLI to resolve owner names (user or org) during module operations.
	t.Run("GetOwners", func(t *testing.T) {
		r := connectRequest(t, "buf.registry.owner.v1.OwnerService/GetOwners", map[string]any{
			"ownerRefs": []map[string]string{{"name": "admin"}},
		}, token)
		owners, ok := r["owners"].([]any)
		if !ok || len(owners) == 0 {
			t.Fatalf("expected owners, got: %v", r)
		}
	})
}
