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

// TestDBStorageBackend boots a separate server that uses DBStore (blobs in SQLite)
// instead of the filesystem, and verifies the full upload→download cycle works.
func TestDBStorageBackend(t *testing.T) {
	// --- Setup: separate server with DBStore ---
	db, err := dbsql.Connect("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	repos := db.Repos()
	az, _ := authz.New(repos)
	repos.Auth = az

	// Use DBStore backed by the same SQLite DB
	store, err := storage.NewDBStore(db.GormDB())
	if err != nil {
		t.Fatal(err)
	}

	// Seed a user
	hash, _ := auth.HashPassword("password123")
	repos.Users.Create(context.Background(), &model.User{
		ID: "dbstore-user", Username: "dbstoreuser", PasswordHash: hash,
		CreatedAt: time.Now().UTC(),
	})

	mux := http.NewServeMux()
	interceptors := connect.WithInterceptors(auth.NewConnectInterceptor(repos))

	p, h := ownerv1connect.NewOwnerServiceHandler(owner.NewOwnerService(repos), interceptors)
	mux.Handle(p, h)
	p, h = modulev1connect.NewModuleServiceHandler(apimodule.NewModuleService(repos), interceptors)
	mux.Handle(p, h)
	p, h = modulev1connect.NewUploadServiceHandler(apimodule.NewUploadService(repos, store), interceptors)
	mux.Handle(p, h)
	p, h = modulev1connect.NewDownloadServiceHandler(apimodule.NewDownloadService(repos, store), interceptors)
	mux.Handle(p, h)

	rest.NewAuthHandler(repos, true).Register(mux)
	rest.NewModuleHandler(repos).Register(mux)

	webFS, _ := fs.Sub(web.Content, "dist")
	mux.Handle("/", http.FileServer(http.FS(webFS)))
	handler := auth.Middleware(repos)(mux)

	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	dbBaseURL := "http://" + listener.Addr().String()
	srv := &http.Server{Handler: handler}
	go srv.Serve(listener)
	defer srv.Close()

	// --- Helper: make requests against this server ---
	doReq := func(method, path string, body any, token string) map[string]any {
		t.Helper()
		var br io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			br = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, dbBaseURL+path, br)
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

	// --- Test: login, create module, push, download ---
	loginResp := doReq("POST", "/api/v1/auth/login", map[string]string{
		"username": "dbstoreuser", "password": "password123",
	}, "")
	token := loginResp["token"].(string)

	doReq("POST", "/api/v1/modules", map[string]string{
		"owner": "dbstoreuser", "name": "dbmod", "visibility": "public",
	}, token)

	proto := base64.StdEncoding.EncodeToString([]byte(
		`syntax = "proto3"; package db.v1; message DBTest { string val = 1; }`,
	))
	upload := doReq("POST", "/buf.registry.module.v1.UploadService/Upload", map[string]any{
		"contents": []map[string]any{{
			"moduleRef": map[string]any{"name": map[string]string{"owner": "dbstoreuser", "module": "dbmod"}},
			"files":     []map[string]string{{"path": "db/v1/test.proto", "content": proto}},
		}},
	}, token)
	if upload["commits"] == nil {
		t.Fatalf("upload to DB-backed store failed: %v", upload)
	}

	dl := doReq("POST", "/buf.registry.module.v1.DownloadService/Download", map[string]any{
		"values": []map[string]any{{
			"resourceRef": map[string]any{"name": map[string]string{"owner": "dbstoreuser", "module": "dbmod"}},
		}},
	}, token)
	contents := dl["contents"].([]any)
	files := contents[0].(map[string]any)["files"].([]any)
	b64 := files[0].(map[string]any)["content"].(string)
	decoded, _ := base64.StdEncoding.DecodeString(b64)
	if !bytes.Contains(decoded, []byte("DBTest")) {
		t.Fatalf("downloaded content from DB store should contain 'DBTest', got: %s", string(decoded))
	}

	t.Log("DB-backed storage: upload + download verified")
}
