package config

import "os"

type Config struct {
	Port        string
	DBDriver    string
	MongoURI    string
	PostgresDSN string
	StoragePath string
	AdminUser   string
	AdminPass   string
	OpenReg       bool
	TLSCert       string
	TLSKey        string
	SQLitePath    string
	StorageDriver string
	S3Endpoint    string
	S3Region      string
	S3Bucket      string
	S3Prefix      string
	S3AccessKey   string
	S3SecretKey   string
}

func Load() *Config {
	return &Config{
		Port:        envOr("BSR_PORT", "8080"),
		DBDriver:    envOr("DB_DRIVER", "sqlite"),
		MongoURI:    envOr("MONGO_URI", "mongodb://localhost:27017/bsr"),
		PostgresDSN: os.Getenv("POSTGRES_DSN"),
		StoragePath: envOr("STORAGE_PATH", "./data/modules"),
		AdminUser:   os.Getenv("ADMIN_USERNAME"),
		AdminPass:   os.Getenv("ADMIN_PASSWORD"),
		OpenReg:     os.Getenv("OPEN_REGISTRATION") == "true",
		TLSCert:     os.Getenv("TLS_CERT"),
		TLSKey:      os.Getenv("TLS_KEY"),
		SQLitePath:    envOr("SQLITE_PATH", "./data/bsr.db"),
		StorageDriver: envOr("STORAGE_DRIVER", "local"),
		S3Endpoint:    os.Getenv("S3_ENDPOINT"),
		S3Region:      envOr("S3_REGION", "us-east-1"),
		S3Bucket:      envOr("S3_BUCKET", "openbsr"),
		S3Prefix:      os.Getenv("S3_PREFIX"),
		S3AccessKey:   os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:   os.Getenv("S3_SECRET_KEY"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
