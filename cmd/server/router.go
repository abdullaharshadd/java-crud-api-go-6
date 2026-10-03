package main

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/rs/zerolog/log"

	"migrated-app/internal/config"
	"migrated-app/internal/httpapi"
	"migrated-app/internal/model"
	"migrated-app/internal/repository"
	"migrated-app/internal/service"
)

// mysqlDSN converts a mysql:// URL into a go-sql-driver DSN. Values that are
// already in DSN form are returned unchanged.
func mysqlDSN(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(raw), "mysql://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return strings.TrimPrefix(raw, "mysql://")
	}
	var b strings.Builder
	if u.User != nil {
		b.WriteString(u.User.String())
		b.WriteString("@")
	}
	b.WriteString("tcp(")
	b.WriteString(u.Host)
	b.WriteString(")")
	if u.Path != "" {
		b.WriteString(u.Path)
	} else {
		b.WriteString("/")
	}
	if u.RawQuery != "" {
		b.WriteString("?")
		b.WriteString(u.RawQuery)
	}
	return b.String()
}

func buildRouter() http.Handler {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}
	dsnSource := cfg.DatabaseURL
	if dsnSource == "" {
		dsnSource = os.Getenv("DATABASE_URL")
	}
	if dsnSource == "" {
		log.Fatal().Msg("DATABASE_URL is not set")
	}

	dsn := mysqlDSN(dsnSource)
	// Allow explicit DB_HOST/DB_PORT env vars to override the host in the DSN,
	// since the database host differs per environment.
	if host := strings.TrimSpace(os.Getenv("DB_HOST")); host != "" {
		if mc, perr := mysqldrv.ParseDSN(dsn); perr == nil {
			port := strings.TrimSpace(os.Getenv("DB_PORT"))
			if port == "" {
				if _, p, serr := net.SplitHostPort(mc.Addr); serr == nil && p != "" {
					port = p
				} else {
					port = "3306"
				}
			}
			mc.Net = "tcp"
			mc.Addr = net.JoinHostPort(host, port)
			dsn = mc.FormatDSN()
		}
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("open database")
	}

	// Wait for the database to become reachable (DNS/container startup).
	deadline := time.Now().Add(60 * time.Second)
	for {
		pctx, pcancel := context.WithTimeout(context.Background(), 5*time.Second)
		perr := db.PingContext(pctx)
		pcancel()
		if perr == nil {
			break
		}
		if time.Now().After(deadline) {
			log.Fatal().Err(perr).Msg("connect database")
		}
		log.Warn().Err(perr).Msg("database not ready, retrying")
		time.Sleep(2 * time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := model.EnsureUserSchema(ctx, db, model.DialectMySQL); err != nil {
		log.Fatal().Err(err).Msg("init schema")
	}

	repo := repository.NewMySQLUserRepository(db)
	svc, err := service.NewUserService(repo)
	if err != nil {
		log.Fatal().Err(err).Msg("create user service")
	}
	h, err := httpapi.NewUserHandler(svc)
	if err != nil {
		log.Fatal().Err(err).Msg("create user handler")
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	r.Mount("/", h.Routes())

	return r
}