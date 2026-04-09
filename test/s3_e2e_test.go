// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

package test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/module/v1/modulev1connect"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/owner/v1/ownerv1connect"

	apimodule "github.com/thomas-maurice/openbsr/internal/api/module"
	"github.com/thomas-maurice/openbsr/internal/api/owner"
	"github.com/thomas-maurice/openbsr/internal/api/rest"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/authz"
	dbsql "github.com/thomas-maurice/openbsr/internal/db/sql"
	"github.com/thomas-maurice/openbsr/internal/model"
	"github.com/thomas-maurice/openbsr/internal/storage"
	"github.com/thomas-maurice/openbsr/internal/web"
)

// TestS3StorageBackend boots a Garage (S3-compatible) container via Docker,
// creates a bucket and API key, then verifies the full upload→download cycle
// using the S3Store backend.
//
// This test is skipped if Docker is not available or the OPENBSR_TEST_S3 env var is not set.
// Run with: OPENBSR_TEST_S3=1 CGO_ENABLED=1 go test -v -run TestS3Storage ./test/
func TestS3StorageBackend(t *testing.T) {
	if os.Getenv("OPENBSR_TEST_S3") == "" {
		t.Skip("set OPENBSR_TEST_S3=1 to run S3 integration tests (requires Docker)")
	}

	// --- Start Garage container ---
	containerName := "openbsr-test-garage"
	configPath, _ := os.Getwd()
	// The test runs from the repo root/test dir, config is in test/garage/
	garageConfig := configPath + "/garage/garage.toml"
	if _, err := os.Stat(garageConfig); err != nil {
		// Try from repo root
		garageConfig = configPath + "/test/garage/garage.toml"
	}

	// Kill any leftover container
	exec.Command("docker", "rm", "-f", containerName).Run()

	cmd := exec.Command("docker", "run", "-d",
		"--name", containerName,
		"-p", "13900:3900", // S3 API
		"-p", "13903:3903", // Admin API
		"-v", garageConfig+":/etc/garage.toml:ro",
		"dxflrs/garage:v1.0.1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to start Garage: %v\n%s", err, string(out))
	}
	t.Cleanup(func() {
		exec.Command("docker", "rm", "-f", containerName).Run()
	})

	// Wait for Garage to be ready
	time.Sleep(2 * time.Second)

	// --- Bootstrap: create layout, bucket, key via setup.sh ---
	setupScript := configPath + "/garage/setup.sh"
	if _, err := os.Stat(setupScript); err != nil {
		setupScript = configPath + "/test/garage/setup.sh"
	}

	setupCmd := exec.Command("bash", setupScript)
	setupCmd.Env = append(os.Environ(),
		"GARAGE_ADMIN=http://localhost:13903",
		"GARAGE_TOKEN=s3cr3t",
	)
	setupOut, err := setupCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Garage setup failed: %v\n%s", err, string(setupOut))
	}

	// Parse access key and secret from setup output
	var accessKey, secretKey string
	for _, line := range strings.Split(string(setupOut), "\n") {
		if strings.HasPrefix(line, "ACCESS_KEY=") {
			accessKey = strings.TrimPrefix(line, "ACCESS_KEY=")
		}
		if strings.HasPrefix(line, "SECRET_KEY=") {
			secretKey = strings.TrimPrefix(line, "SECRET_KEY=")
		}
	}
	if accessKey == "" || secretKey == "" {
		t.Fatalf("failed to get Garage credentials from setup output:\n%s", string(setupOut))
	}
	t.Logf("Garage credentials: access=%s secret=%s", accessKey, secretKey[:8]+"...")

	// --- Create S3Store ---
	s3Store, err := storage.NewS3Store(
		"http://localhost:13900", // Garage S3 endpoint
		"garage",                // region
		"openbsr",               // bucket
		"",                      // no prefix
		accessKey,
		secretKey,
	)
	if err != nil {
		t.Fatal(err)
	}

	// --- Boot a test server with S3 storage ---
	sqlDB, _ := dbsql.Connect("sqlite", ":memory:")
	repos := sqlDB.Repos()
	az, _ := authz.New(repos)
	repos.Auth = az

	hash, _ := auth.HashPassword("password123")
	repos.Users.Create(context.Background(), &model.User{
		ID: "s3-user", Username: "s3user", PasswordHash: hash,
		CreatedAt: time.Now().UTC(),
	})

	mux := http.NewServeMux()
	interceptors := connect.WithInterceptors(auth.NewConnectInterceptor(repos))

	p, h := ownerv1connect.NewOwnerServiceHandler(owner.NewOwnerService(repos), interceptors)
	mux.Handle(p, h)
	p, h = modulev1connect.NewModuleServiceHandler(apimodule.NewModuleService(repos), interceptors)
	mux.Handle(p, h)
	p, h = modulev1connect.NewUploadServiceHandler(apimodule.NewUploadService(repos, s3Store), interceptors)
	mux.Handle(p, h)
	p, h = modulev1connect.NewDownloadServiceHandler(apimodule.NewDownloadService(repos, s3Store), interceptors)
	mux.Handle(p, h)

	rest.NewAuthHandler(repos, true).Register(mux)
	rest.NewModuleHandler(repos).Register(mux)

	webFS, _ := fs.Sub(web.Content, "dist")
	mux.Handle("/", http.FileServer(http.FS(webFS)))
	handler := auth.Middleware(repos)(mux)

	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	s3BaseURL := "http://" + listener.Addr().String()
	srv := &http.Server{Handler: handler}
	go srv.Serve(listener)
	defer srv.Close()

	// --- Helper ---
	doReq := func(method, path string, body any, token string) map[string]any {
		t.Helper()
		var br io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			br = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, s3BaseURL+path, br)
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		if result == nil {
			result = map[string]any{}
		}
		result["_status"] = float64(resp.StatusCode)
		return result
	}

	// --- Test: login, create module, push, download via S3 ---
	loginResp := doReq("POST", "/api/v1/auth/login", map[string]string{
		"username": "s3user", "password": "password123",
	}, "")
	token := loginResp["token"].(string)

	doReq("POST", "/api/v1/modules", map[string]string{
		"owner": "s3user", "name": "s3mod", "visibility": "public",
	}, token)

	proto := base64.StdEncoding.EncodeToString([]byte(
		`syntax = "proto3"; package s3.v1; message S3Test { string bucket = 1; }`,
	))
	upload := doReq("POST", "/buf.registry.module.v1.UploadService/Upload", map[string]any{
		"contents": []map[string]any{{
			"moduleRef": map[string]any{"name": map[string]string{"owner": "s3user", "module": "s3mod"}},
			"files":     []map[string]string{{"path": "s3/v1/test.proto", "content": proto}},
		}},
	}, token)
	if upload["commits"] == nil {
		t.Fatalf("upload to S3 store failed: %v", upload)
	}
	t.Logf("Uploaded to S3, commit: %v", upload["commits"].([]any)[0].(map[string]any)["id"])

	// Download and verify content
	dl := doReq("POST", "/buf.registry.module.v1.DownloadService/Download", map[string]any{
		"values": []map[string]any{{
			"resourceRef": map[string]any{"name": map[string]string{"owner": "s3user", "module": "s3mod"}},
		}},
	}, token)
	contents := dl["contents"].([]any)
	files := contents[0].(map[string]any)["files"].([]any)
	b64 := files[0].(map[string]any)["content"].(string)
	decoded, _ := base64.StdEncoding.DecodeString(b64)
	if !bytes.Contains(decoded, []byte("S3Test")) {
		t.Fatalf("downloaded content from S3 should contain 'S3Test', got: %s", string(decoded))
	}

	t.Log("S3-backed storage (Garage): upload + download verified")
}
