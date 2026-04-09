// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

package test

import (
	"bytes"
	"io"
	"testing"
)

// TestModuleLifecycle tests module CRUD: create, get, list, search, duplicate rejection.
func TestModuleLifecycle(t *testing.T) {
	register(t, "moduser", "password123")
	token := login(t, "moduser", "password123")

	// Create a public module
	r := apiRequest(t, "POST", "/api/v1/modules", map[string]string{
		"owner": "moduser", "name": "testmod", "visibility": "public",
	}, token)
	assertStatus(t, r, 201, "create module")
	if r["name"] != "testmod" {
		t.Fatalf("module name mismatch: %v", r)
	}

	// Get module by owner/name (no auth needed for public)
	r = apiRequest(t, "GET", "/api/v1/modules/moduser/testmod", nil, "")
	if r["name"] != "testmod" {
		t.Fatalf("get module: %v", r)
	}

	// List modules by owner
	mods := apiRequestArray(t, "GET", "/api/v1/modules?owner=moduser", "")
	if len(mods) != 1 || mods[0]["name"] != "testmod" {
		t.Fatalf("list modules by owner: %v", mods)
	}

	// Search public modules by name
	mods = apiRequestArray(t, "GET", "/api/v1/modules?q=testmod", "")
	found := false
	for _, m := range mods {
		if m["name"] == "testmod" && m["owner"] == "moduser" {
			found = true
		}
	}
	if !found {
		t.Fatalf("search: testmod not found in %v", mods)
	}

	// Duplicate creation returns 409
	r = apiRequest(t, "POST", "/api/v1/modules", map[string]string{
		"owner": "moduser", "name": "testmod", "visibility": "public",
	}, token)
	assertStatus(t, r, 409, "duplicate module")
}

// TestPrivateModuleAccess verifies that private modules are hidden from
// unauthorized users and unauthenticated requests, but visible to the owner.
func TestPrivateModuleAccess(t *testing.T) {
	register(t, "privowner", "password123")
	register(t, "stranger", "password123")
	ownerToken := login(t, "privowner", "password123")
	strangerToken := login(t, "stranger", "password123")

	// Create a private module
	apiRequest(t, "POST", "/api/v1/modules", map[string]string{
		"owner": "privowner", "name": "secret", "visibility": "private",
	}, ownerToken)

	// Owner can access it
	r := apiRequest(t, "GET", "/api/v1/modules/privowner/secret", nil, ownerToken)
	if r["name"] != "secret" {
		t.Fatalf("owner should see private module: %v", r)
	}

	// Another authenticated user gets 404 (not 403 — we don't reveal existence)
	r = apiRequest(t, "GET", "/api/v1/modules/privowner/secret", nil, strangerToken)
	assertStatus(t, r, 404, "stranger sees private module")

	// Unauthenticated gets 404
	r = apiRequest(t, "GET", "/api/v1/modules/privowner/secret", nil, "")
	assertStatus(t, r, 404, "unauthed sees private module")
}

// TestHealthz verifies the health check endpoint returns 200.
func TestHealthz(t *testing.T) {
	resp, err := client.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("healthz: %d", resp.StatusCode)
	}
}

// TestUIServes verifies the Vue SPA is embedded and served at /.
func TestUIServes(t *testing.T) {
	resp, err := client.Get(baseURL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("UI status: %d", resp.StatusCode)
	}
	if !bytes.Contains(body, []byte("OpenBSR")) {
		t.Fatal("index.html does not contain 'OpenBSR'")
	}
}
