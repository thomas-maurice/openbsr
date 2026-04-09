// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

package test

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// TestUploadDownload exercises the full push/pull lifecycle:
//
//  1. Push v1 of a proto file → get a content-addressable commit hash
//  2. Push v2 with a new field → get a different commit hash
//  3. Download latest → should return v2 content
//  4. Download v1 by commit ref → should return original content
//  5. Verify commit list has 2 entries
//  6. Verify "main" label points to v2
//  7. Re-push v2 (same content) → idempotent, no new commit created
func TestUploadDownload(t *testing.T) {
	register(t, "pushuser", "password123")
	token := login(t, "pushuser", "password123")

	apiRequest(t, "POST", "/api/v1/modules", map[string]string{
		"owner": "pushuser", "name": "protomod", "visibility": "public",
	}, token)

	// --- Push v1 ---
	protoV1 := base64.StdEncoding.EncodeToString([]byte(
		`syntax = "proto3"; package test.v1; message Foo { string name = 1; }`,
	))
	upload1 := connectRequest(t, "buf.registry.module.v1.UploadService/Upload", map[string]any{
		"contents": []map[string]any{{
			"moduleRef": map[string]any{"name": map[string]string{"owner": "pushuser", "module": "protomod"}},
			"files":     []map[string]string{{"path": "test/v1/foo.proto", "content": protoV1}},
		}},
	}, token)
	commit1 := upload1["commits"].([]any)[0].(map[string]any)["id"].(string)
	if commit1 == "" {
		t.Fatal("v1 upload returned no commit ID")
	}

	// --- Push v2 (adds a "bar" field) ---
	protoV2 := base64.StdEncoding.EncodeToString([]byte(
		`syntax = "proto3"; package test.v1; message Foo { string name = 1; string bar = 2; }`,
	))
	upload2 := connectRequest(t, "buf.registry.module.v1.UploadService/Upload", map[string]any{
		"contents": []map[string]any{{
			"moduleRef": map[string]any{"name": map[string]string{"owner": "pushuser", "module": "protomod"}},
			"files":     []map[string]string{{"path": "test/v1/foo.proto", "content": protoV2}},
		}},
	}, token)
	commit2 := upload2["commits"].([]any)[0].(map[string]any)["id"].(string)
	if commit1 == commit2 {
		t.Fatal("v1 and v2 should produce different commit hashes (content-addressable)")
	}

	// --- Download latest (should be v2) ---
	dl := connectRequest(t, "buf.registry.module.v1.DownloadService/Download", map[string]any{
		"values": []map[string]any{{
			"resourceRef": map[string]any{"name": map[string]string{"owner": "pushuser", "module": "protomod"}},
		}},
	}, token)
	latestContent := decodeFileContent(t, dl)
	if !bytes.Contains(latestContent, []byte("bar")) {
		t.Fatalf("latest download should be v2 (has 'bar' field), got: %s", string(latestContent))
	}

	// --- Download v1 by commit ref ---
	dlV1 := connectRequest(t, "buf.registry.module.v1.DownloadService/Download", map[string]any{
		"values": []map[string]any{{
			"resourceRef": map[string]any{"name": map[string]any{
				"owner": "pushuser", "module": "protomod", "ref": commit1,
			}},
		}},
	}, token)
	v1Content := decodeFileContent(t, dlV1)
	if bytes.Contains(v1Content, []byte("bar")) {
		t.Fatal("v1 download should NOT contain 'bar' field")
	}

	// --- Verify commit list ---
	commits := apiRequestArray(t, "GET", "/api/v1/modules/pushuser/protomod/commits", token)
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}

	// --- Verify main label points to v2 ---
	labels := apiRequestArray(t, "GET", "/api/v1/modules/pushuser/protomod/labels", token)
	if len(labels) != 1 || labels[0]["name"] != "main" {
		t.Fatalf("expected single 'main' label, got %v", labels)
	}
	if labels[0]["commit_id"] != commit2 {
		t.Fatal("main label should point to v2 commit")
	}

	// --- Idempotent push: re-push v2 → same hash, no new commit ---
	upload3 := connectRequest(t, "buf.registry.module.v1.UploadService/Upload", map[string]any{
		"contents": []map[string]any{{
			"moduleRef": map[string]any{"name": map[string]string{"owner": "pushuser", "module": "protomod"}},
			"files":     []map[string]string{{"path": "test/v1/foo.proto", "content": protoV2}},
		}},
	}, token)
	commit3 := upload3["commits"].([]any)[0].(map[string]any)["id"].(string)
	if commit3 != commit2 {
		t.Fatal("re-pushing identical content should return the same commit hash")
	}
	commitsAfter := apiRequestArray(t, "GET", "/api/v1/modules/pushuser/protomod/commits", token)
	if len(commitsAfter) != 2 {
		t.Fatalf("idempotent push should not create a new commit, got %d", len(commitsAfter))
	}
}

// TestUploadUnauthenticated verifies that pushing without a token is rejected.
func TestUploadUnauthenticated(t *testing.T) {
	r := connectRequest(t, "buf.registry.module.v1.UploadService/Upload", map[string]any{
		"contents": []map[string]any{{}},
	}, "")
	if r["code"] != "unauthenticated" {
		t.Fatalf("expected unauthenticated error, got: %v", r)
	}
}

// decodeFileContent extracts and base64-decodes the first file's content from a Download response.
func decodeFileContent(t *testing.T, dl map[string]any) []byte {
	t.Helper()
	contents := dl["contents"].([]any)
	files := contents[0].(map[string]any)["files"].([]any)
	b64 := files[0].(map[string]any)["content"].(string)
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	return decoded
}
