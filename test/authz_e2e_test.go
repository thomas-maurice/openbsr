// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

package test

import (
	"encoding/base64"
	"testing"
)

// TestOrgModuleAuthz verifies the role-based authorization boundaries for
// organization-owned modules:
//
//   - Org admin can create modules under the org
//   - Org member CANNOT create modules (requires admin)
//   - Org member CAN push content to existing org modules
//   - Non-member (outsider) CANNOT push to org modules
func TestOrgModuleAuthz(t *testing.T) {
	register(t, "orgadmin2", "password123")
	register(t, "orgmember2", "password123")
	register(t, "outsider2", "password123")
	adminToken := login(t, "orgadmin2", "password123")
	memberToken := login(t, "orgmember2", "password123")
	outsiderToken := login(t, "outsider2", "password123")

	// Setup: create org, add member
	apiRequest(t, "POST", "/api/v1/orgs", map[string]string{"name": "authz-org"}, adminToken)
	apiRequest(t, "POST", "/api/v1/orgs/authz-org/members",
		map[string]string{"username": "orgmember2", "role": "member"}, adminToken)

	// Admin can create module under org
	r := apiRequest(t, "POST", "/api/v1/modules", map[string]string{
		"owner": "authz-org", "name": "orgmod", "visibility": "public",
	}, adminToken)
	assertStatus(t, r, 201, "admin creates org module")

	// Member cannot create module under org (requires admin role)
	r = apiRequest(t, "POST", "/api/v1/modules", map[string]string{
		"owner": "authz-org", "name": "orgmod2", "visibility": "public",
	}, memberToken)
	assertStatus(t, r, 403, "member creates org module")

	// Member CAN push content to existing org module
	proto := base64.StdEncoding.EncodeToString([]byte(`syntax = "proto3"; package x.v1; message X {}`))
	upload := connectRequest(t, "buf.registry.module.v1.UploadService/Upload", map[string]any{
		"contents": []map[string]any{{
			"moduleRef": map[string]any{"name": map[string]string{"owner": "authz-org", "module": "orgmod"}},
			"files":     []map[string]string{{"path": "x/v1/x.proto", "content": proto}},
		}},
	}, memberToken)
	if upload["commits"] == nil {
		t.Fatalf("org member should be able to push: %v", upload)
	}

	// Outsider (not a member) CANNOT push
	uploadFail := connectRequest(t, "buf.registry.module.v1.UploadService/Upload", map[string]any{
		"contents": []map[string]any{{
			"moduleRef": map[string]any{"name": map[string]string{"owner": "authz-org", "module": "orgmod"}},
			"files":     []map[string]string{{"path": "x/v1/x.proto", "content": proto}},
		}},
	}, outsiderToken)
	if uploadFail["code"] != "permission_denied" {
		t.Fatalf("outsider should be denied push access: %v", uploadFail)
	}
}
