// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

package test

import "testing"

// TestOrgLifecycle tests the full organization lifecycle:
// create org → get org → add member → verify non-admin cannot add → remove member.
func TestOrgLifecycle(t *testing.T) {
	register(t, "orgowner", "password123")
	register(t, "orgmember", "password123")
	ownerToken := login(t, "orgowner", "password123")
	memberToken := login(t, "orgmember", "password123")

	// Create an organization — the creator automatically becomes admin
	r := apiRequest(t, "POST", "/api/v1/orgs", map[string]string{"name": "test-org"}, ownerToken)
	if r["name"] != "test-org" {
		t.Fatalf("create org failed: %v", r)
	}

	// Public org lookup (no auth required)
	r = apiRequest(t, "GET", "/api/v1/orgs/test-org", nil, "")
	if r["name"] != "test-org" {
		t.Fatalf("get org failed: %v", r)
	}

	// Admin adds a member
	r = apiRequest(t, "POST", "/api/v1/orgs/test-org/members",
		map[string]string{"username": "orgmember", "role": "member"}, ownerToken)
	if r["status"] != "added" {
		t.Fatalf("add member failed: %v", r)
	}

	// Non-admin member tries to add someone — should be 403
	r = apiRequest(t, "POST", "/api/v1/orgs/test-org/members",
		map[string]string{"username": "orgowner", "role": "member"}, memberToken)
	assertStatus(t, r, 403, "non-admin add member")

	// Admin removes the member
	r = apiRequest(t, "DELETE", "/api/v1/orgs/test-org/members/orgmember", nil, ownerToken)
	if r["status"] != "removed" {
		t.Fatalf("remove member failed: %v", r)
	}
}
