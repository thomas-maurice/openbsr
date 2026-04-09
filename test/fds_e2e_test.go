// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

package test

import (
	"encoding/base64"
	"testing"
)

// TestFileDescriptorSet verifies that the server can compile uploaded .proto files
// into a FileDescriptorSet using bufbuild/protocompile and return it via ConnectRPC.
func TestFileDescriptorSet(t *testing.T) {
	register(t, "fdsuser", "password123")
	token := login(t, "fdsuser", "password123")

	// Create module and upload a proto with a service definition
	apiRequest(t, "POST", "/api/v1/modules", map[string]string{
		"owner": "fdsuser", "name": "fdsmod", "visibility": "public",
	}, token)

	proto := base64.StdEncoding.EncodeToString([]byte(
		`syntax = "proto3"; package fds.v1; ` +
			`message PingRequest { string msg = 1; } ` +
			`message PingResponse { string reply = 1; } ` +
			`service PingService { rpc Ping(PingRequest) returns (PingResponse); }`,
	))
	connectRequest(t, "buf.registry.module.v1.UploadService/Upload", map[string]any{
		"contents": []map[string]any{{
			"moduleRef": map[string]any{"name": map[string]string{"owner": "fdsuser", "module": "fdsmod"}},
			"files":     []map[string]string{{"path": "fds/v1/ping.proto", "content": proto}},
		}},
	}, token)

	// Fetch the compiled FileDescriptorSet
	fds := connectRequest(t, "buf.registry.module.v1.FileDescriptorSetService/GetFileDescriptorSet", map[string]any{
		"resourceRef": map[string]any{"name": map[string]string{"owner": "fdsuser", "module": "fdsmod"}},
	}, token)
	if fds["fileDescriptorSet"] == nil {
		t.Fatalf("expected fileDescriptorSet in response, got: %v", fds)
	}

	// Public modules should be accessible without auth
	fdsNoAuth := connectRequest(t, "buf.registry.module.v1.FileDescriptorSetService/GetFileDescriptorSet", map[string]any{
		"resourceRef": map[string]any{"name": map[string]string{"owner": "fdsuser", "module": "fdsmod"}},
	}, "")
	if fdsNoAuth["fileDescriptorSet"] == nil {
		t.Fatal("public module FDS should work without authentication")
	}
}
