package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/go-sql-driver/mysql"
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

	db, err := sql.Open("mysql", mysqlDSN(dsnSource))
	if err != nil {
		log.Fatal().Err(err).Msg("open database")
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