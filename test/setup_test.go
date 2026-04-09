// Copyright (c) 2026 Thomas Maurice
// SPDX-License-Identifier: MIT

// Package test provides end-to-end tests for OpenBSR.
//
// All tests share a single in-process server backed by SQLite :memory:.
// The server is created once in TestMain and torn down when all tests finish.
// No Docker, no external services — runs in ~1.5 seconds.
//
// Test files are organized by feature area:
//   - setup_test.go    — Server bootstrap and test helpers
//   - auth_e2e_test.go — Registration, login, tokens
//   - org_e2e_test.go  — Organization CRUD and member management
//   - module_e2e_test.go — Module CRUD, visibility, search
//   - push_pull_e2e_test.go — Upload, download, commits, labels, idempotent push
//   - fds_e2e_test.go  — FileDescriptorSet (proto reflection)
//   - authz_e2e_test.go — Authorization boundaries (org roles, write access)
//   - connectrpc_e2e_test.go — ConnectRPC service smoke tests
package test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"buf.build/gen/go/bufbuild/buf/connectrpc/go/buf/alpha/registry/v1alpha1/registryv1alpha1connect"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/module/v1/modulev1connect"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/owner/v1/ownerv1connect"

	"github.com/thomas-maurice/openbsr/internal/api/authn"
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

// baseURL is the address of the in-process test server (e.g. "http://127.0.0.1:54321").
var baseURL string

// client is the shared HTTP client for all test requests.
var client *http.Client

// TestMain boots a full OpenBSR server in-process with SQLite :memory:.
// Every E2E test in this package runs against this single server instance.
func TestMain(m *testing.M) {
	// --- Database ---
	db, err := dbsql.Connect("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	repos := db.Repos()

	// --- Authorization ---
	az, err := authz.New(repos)
	if err != nil {
		panic(err)
	}
	repos.Auth = az

	// --- Blob storage (temp dir, cleaned up after tests) ---
	tmpDir, _ := os.MkdirTemp("", "bsr-e2e-*")
	defer os.RemoveAll(tmpDir)
	store := storage.NewLocalStore(tmpDir)

	// --- Seed admin user ---
	hash, _ := auth.HashPassword("changeme")
	repos.Users.Create(context.Background(), &model.User{
		ID: "admin-admin", Username: "admin", PasswordHash: hash,
		CreatedAt: time.Now().UTC(),
	})

	// --- HTTP Mux: ConnectRPC services ---
	mux := http.NewServeMux()
	interceptors := connect.WithInterceptors(auth.NewConnectInterceptor(repos))

	svcAuthn := authn.NewAuthnService()
	p, h := registryv1alpha1connect.NewAuthnServiceHandler(svcAuthn, interceptors)
	mux.Handle(p, h)

	svcOwner := owner.NewOwnerService(repos)
	p, h = ownerv1connect.NewOwnerServiceHandler(svcOwner, interceptors)
	mux.Handle(p, h)
	svcOrg := owner.NewOrganizationService(repos)
	p, h = ownerv1connect.NewOrganizationServiceHandler(svcOrg, interceptors)
	mux.Handle(p, h)
	svcUser := owner.NewUserService(repos, true)
	p, h = ownerv1connect.NewUserServiceHandler(svcUser, interceptors)
	mux.Handle(p, h)

	svcMod := apimodule.NewModuleService(repos)
	p, h = modulev1connect.NewModuleServiceHandler(svcMod, interceptors)
	mux.Handle(p, h)
	svcUpload := apimodule.NewUploadService(repos, store)
	p, h = modulev1connect.NewUploadServiceHandler(svcUpload, interceptors)
	mux.Handle(p, h)
	svcDownload := apimodule.NewDownloadService(repos, store)
	p, h = modulev1connect.NewDownloadServiceHandler(svcDownload, interceptors)
	mux.Handle(p, h)
	svcCommit := apimodule.NewCommitService(repos)
	p, h = modulev1connect.NewCommitServiceHandler(svcCommit, interceptors)
	mux.Handle(p, h)
	svcLabel := apimodule.NewLabelService(repos)
	p, h = modulev1connect.NewLabelServiceHandler(svcLabel, interceptors)
	mux.Handle(p, h)
	svcFDS := apimodule.NewFileDescriptorSetService(repos, store)
	p, h = modulev1connect.NewFileDescriptorSetServiceHandler(svcFDS, interceptors)
	mux.Handle(p, h)

	// --- HTTP Mux: REST handlers ---
	rest.NewAuthHandler(repos, true).Register(mux)
	rest.NewOrgHandler(repos).Register(mux)
	rest.NewUserHandler(repos).Register(mux)
	rest.NewModuleHandler(repos).Register(mux)
	rest.NewCommitHandler(repos).Register(mux)
	rest.NewFileHandler(repos, store).Register(mux)

	// --- HTTP Mux: Health + Web UI ---
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	webFS, _ := fs.Sub(web.Content, "dist")
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	// --- Start server ---
	handler := auth.Middleware(repos)(mux)
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	baseURL = "http://" + listener.Addr().String()
	srv := &http.Server{Handler: handler}
	go srv.Serve(listener)
	defer srv.Close()

	client = &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}

	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// Test helpers — used by all E2E test files
// ---------------------------------------------------------------------------

// apiRequest sends a REST API request and returns the JSON response as a map.
// The "_status" key is injected with the HTTP status code as a float64.
func apiRequest(t *testing.T, method, path string, body any, token string) map[string]any {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, baseURL+path, bodyReader)
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

// apiRequestArray sends a REST API request and returns the JSON response as an array of maps.
func apiRequestArray(t *testing.T, method, path string, token string) []map[string]any {
	t.Helper()
	req, _ := http.NewRequest(method, baseURL+path, nil)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result []map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

// connectRequest sends a ConnectRPC (JSON) request and returns the response as a map.
// path is the full RPC path, e.g. "buf.registry.module.v1.UploadService/Upload".
func connectRequest(t *testing.T, path string, body any, token string) map[string]any {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", baseURL+"/"+path, bytes.NewReader(b))
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
	return result
}

// register creates a new user account. Ignores errors (user may already exist).
func register(t *testing.T, username, password string) {
	t.Helper()
	apiRequest(t, "POST", "/api/v1/auth/register", map[string]string{
		"username": username, "password": password,
	}, "")
}

// login authenticates a user and returns the raw Bearer token.
func login(t *testing.T, username, password string) string {
	t.Helper()
	r := apiRequest(t, "POST", "/api/v1/auth/login", map[string]string{
		"username": username, "password": password,
	}, "")
	token, _ := r["token"].(string)
	if token == "" {
		t.Fatalf("login failed for %s: %v", username, r)
	}
	return token
}

// assertStatus checks that the response has the expected HTTP status code.
func assertStatus(t *testing.T, r map[string]any, expected int, msg string) {
	t.Helper()
	if r["_status"] != float64(expected) {
		t.Fatalf("%s: expected %d, got %v — response: %v", msg, expected, r["_status"], r)
	}
}
