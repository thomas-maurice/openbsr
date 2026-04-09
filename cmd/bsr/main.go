package main

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"connectrpc.com/connect"
	"buf.build/gen/go/bufbuild/buf/connectrpc/go/buf/alpha/registry/v1alpha1/registryv1alpha1connect"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/module/v1/modulev1connect"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/owner/v1/ownerv1connect"
	charmlog "github.com/charmbracelet/log"
	"github.com/thomas-maurice/openbsr/internal/api/authn"
	apimodule "github.com/thomas-maurice/openbsr/internal/api/module"
	"github.com/thomas-maurice/openbsr/internal/api/owner"
	"github.com/thomas-maurice/openbsr/internal/api/rest"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/config"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/authz"
	dbmongo "github.com/thomas-maurice/openbsr/internal/db/mongo"
	dbsql "github.com/thomas-maurice/openbsr/internal/db/sql"
	"github.com/thomas-maurice/openbsr/internal/model"
	"github.com/thomas-maurice/openbsr/internal/storage"
	"github.com/thomas-maurice/openbsr/internal/web"
)

func main() {
	handler := charmlog.NewWithOptions(os.Stderr, charmlog.Options{Level: charmlog.DebugLevel})
	slog.SetDefault(slog.New(handler))

	cfg := config.Load()
	slog.Info("starting openbsr", "port", cfg.Port, "db", cfg.DBDriver)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var repos *iface.Repos
	var dbClose func()
	var sqlDB *dbsql.DB // non-nil when using SQL backend (for DB blob storage)

	switch cfg.DBDriver {
	case "mongo":
		db, err := dbmongo.Connect(ctx, cfg.MongoURI)
		if err != nil {
			slog.Error("failed to connect to mongodb", "err", err)
			os.Exit(1)
		}
		dbClose = func() { db.Close(context.Background()) }
		repos = db.Repos()
		slog.Info("connected to mongodb")
	case "sqlite":
		os.MkdirAll("./data", 0755)
		db, err := dbsql.Connect("sqlite", cfg.SQLitePath)
		if err != nil {
			slog.Error("failed to open sqlite", "err", err, "path", cfg.SQLitePath)
			os.Exit(1)
		}
		dbClose = func() { db.Close() }
		repos = db.Repos()
		sqlDB = db
		slog.Info("connected to sqlite", "path", cfg.SQLitePath)
	case "postgres":
		db, err := dbsql.Connect("postgres", cfg.PostgresDSN)
		if err != nil {
			slog.Error("failed to connect to postgres", "err", err)
			os.Exit(1)
		}
		dbClose = func() { db.Close() }
		repos = db.Repos()
		sqlDB = db
		slog.Info("connected to postgres")
	default:
		slog.Error("unsupported DB_DRIVER", "driver", cfg.DBDriver)
		os.Exit(1)
	}
	defer dbClose()

	// --- Storage backend ---
	var store storage.Store
	switch cfg.StorageDriver {
	case "local", "":
		os.MkdirAll(cfg.StoragePath, 0755)
		store = storage.NewLocalStore(cfg.StoragePath)
		slog.Info("storage initialized", "driver", "local", "path", cfg.StoragePath)
	case "s3":
		s, err := storage.NewS3Store(cfg.S3Endpoint, cfg.S3Region, cfg.S3Bucket, cfg.S3Prefix, cfg.S3AccessKey, cfg.S3SecretKey)
		if err != nil {
			slog.Error("failed to create S3 store", "err", err)
			os.Exit(1)
		}
		store = s
		slog.Info("storage initialized", "driver", "s3", "bucket", cfg.S3Bucket, "endpoint", cfg.S3Endpoint)
	case "db":
		if sqlDB == nil {
			slog.Error("STORAGE_DRIVER=db requires DB_DRIVER=sqlite or postgres")
			os.Exit(1)
		}
		s, err := storage.NewDBStore(sqlDB.GormDB())
		if err != nil {
			slog.Error("failed to create DB store", "err", err)
			os.Exit(1)
		}
		store = s
		slog.Info("storage initialized", "driver", "db")
	default:
		slog.Error("unsupported STORAGE_DRIVER", "driver", cfg.StorageDriver)
		os.Exit(1)
	}

	az, err := authz.New(repos)
	if err != nil {
		slog.Error("failed to create authorizer", "err", err)
		os.Exit(1)
	}
	repos.Auth = az

	// Ensure admin user exists
	if cfg.AdminUser != "" && cfg.AdminPass != "" {
		ensureAdmin(ctx, repos, cfg.AdminUser, cfg.AdminPass)
	}

	mux := http.NewServeMux()

	// ConnectRPC services (with auth interceptor)
	interceptors := connect.WithInterceptors(auth.NewConnectInterceptor(repos))

	// AuthnService (buf.alpha.registry.v1alpha1)
	authnSvc := authn.NewAuthnService()
	authnPath, authnHandler := registryv1alpha1connect.NewAuthnServiceHandler(authnSvc, interceptors)
	mux.Handle(authnPath, authnHandler)

	// OwnerService, OrganizationService, UserService (buf.registry.owner.v1)
	ownerSvc := owner.NewOwnerService(repos)
	ownerPath, ownerHandler := ownerv1connect.NewOwnerServiceHandler(ownerSvc, interceptors)
	mux.Handle(ownerPath, ownerHandler)

	orgSvc := owner.NewOrganizationService(repos)
	orgPath, orgHandler := ownerv1connect.NewOrganizationServiceHandler(orgSvc, interceptors)
	mux.Handle(orgPath, orgHandler)

	userSvc := owner.NewUserService(repos, cfg.OpenReg)
	userPath, userHandler := ownerv1connect.NewUserServiceHandler(userSvc, interceptors)
	mux.Handle(userPath, userHandler)

	// ModuleService (buf.registry.module.v1)
	modSvc := apimodule.NewModuleService(repos)
	modPath, modHandler := modulev1connect.NewModuleServiceHandler(modSvc, interceptors)
	mux.Handle(modPath, modHandler)

	// UploadService, DownloadService (buf.registry.module.v1)
	uploadSvc := apimodule.NewUploadService(repos, store)
	uploadPath, uploadHandler := modulev1connect.NewUploadServiceHandler(uploadSvc, interceptors)
	mux.Handle(uploadPath, uploadHandler)

	downloadSvc := apimodule.NewDownloadService(repos, store)
	downloadPath, downloadHandler := modulev1connect.NewDownloadServiceHandler(downloadSvc, interceptors)
	mux.Handle(downloadPath, downloadHandler)

	// CommitService, LabelService (buf.registry.module.v1)
	commitSvc := apimodule.NewCommitService(repos)
	commitPath, commitHandler := modulev1connect.NewCommitServiceHandler(commitSvc, interceptors)
	mux.Handle(commitPath, commitHandler)

	labelSvc := apimodule.NewLabelService(repos)
	labelPath, labelHandler := modulev1connect.NewLabelServiceHandler(labelSvc, interceptors)
	mux.Handle(labelPath, labelHandler)

	// FileDescriptorSetService (buf.registry.module.v1)
	fdsSvc := apimodule.NewFileDescriptorSetService(repos, store)
	fdsPath, fdsHandler := modulev1connect.NewFileDescriptorSetServiceHandler(fdsSvc, interceptors)
	mux.Handle(fdsPath, fdsHandler)

	// REST API
	authAPIHandler := rest.NewAuthHandler(repos, cfg.OpenReg)
	authAPIHandler.Register(mux)

	orgRESTHandler := rest.NewOrgHandler(repos)
	orgRESTHandler.Register(mux)

	userRESTHandler := rest.NewUserHandler(repos)
	userRESTHandler.Register(mux)

	moduleRESTHandler := rest.NewModuleHandler(repos)
	moduleRESTHandler.Register(mux)

	commitRESTHandler := rest.NewCommitHandler(repos)
	commitRESTHandler.Register(mux)

	fileRESTHandler := rest.NewFileHandler(repos, store)
	fileRESTHandler.Register(mux)

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Embedded Web UI (SPA)
	webFS, err := fs.Sub(web.Content, "dist")
	if err != nil {
		slog.Error("failed to create web filesystem", "err", err)
		os.Exit(1)
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	// Wrap with auth middleware and rate limiting
	httpHandler := auth.Middleware(repos)(mux)
	httpHandler = auth.RateLimitMiddleware(200, time.Minute)(httpHandler)
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: httpHandler,
	}

	go func() {
		if cfg.TLSCert != "" && cfg.TLSKey != "" {
			slog.Info("listening (TLS)", "addr", srv.Addr)
			if err := srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey); err != nil && err != http.ErrServerClosed {
				slog.Error("server error", "err", err)
				os.Exit(1)
			}
		} else {
			slog.Info("listening", "addr", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("server error", "err", err)
				os.Exit(1)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	slog.Info("shutting down")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx)
}

func ensureAdmin(ctx context.Context, repos *iface.Repos, username, password string) {
	_, err := repos.Users.GetByUsername(ctx, username)
	if err == nil {
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		slog.Error("hash admin password", "err", err)
		return
	}
	u := &model.User{
		ID:           "admin-" + username,
		Username:     username,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	if err := repos.Users.Create(ctx, u); err != nil {
		slog.Error("create admin user", "err", err)
		return
	}
	slog.Info("admin user created", "username", username)
}
