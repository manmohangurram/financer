// Command financer is the Financer backend: config → db → services → HTTP server.
package main

import (
	"log/slog"
	"net/http"
	"os"
	stdtime "time"

	"github.com/mohan9182/financer/internal/accountservice"
	"github.com/mohan9182/financer/internal/auth"
	"github.com/mohan9182/financer/internal/authservice"
	"github.com/mohan9182/financer/internal/categoryservice"
	"github.com/mohan9182/financer/internal/config"
	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/keyservice"
	"github.com/mohan9182/financer/internal/logging"
	"github.com/mohan9182/financer/internal/mailservice"
	"github.com/mohan9182/financer/internal/repo/sqlite"
	"github.com/mohan9182/financer/internal/ruleservice"
	"github.com/mohan9182/financer/internal/server"
	"github.com/mohan9182/financer/internal/time"
	"github.com/mohan9182/financer/internal/transactionservice"
	"github.com/mohan9182/financer/internal/transferservice"
	"github.com/mohan9182/financer/internal/userservice"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	closeLogs, err := logging.Setup(cfg.Server.LogLevel, cfg.Server.LogFile)
	if err != nil {
		slog.Error("logging setup failed", "err", err)
		os.Exit(1)
	}
	defer closeLogs()
	log := slog.Default()

	if cfg.Server.AllowInsecureHTTP {
		log.Warn("FINANCER_ALLOW_INSECURE_HTTP is set: credentials may cross the network in plaintext")
	}

	// Set the process timezone once, before any date/time work.
	time.SetLoc(cfg.Server.Timezone)
	if time.Loc.String() != cfg.Server.Timezone {
		log.Warn("timezone not found; falling back", "requested", cfg.Server.Timezone, "using", time.Loc.String())
	}

	log.Info("starting financer",
		"addr", cfg.Server.Addr,
		"data_dir", cfg.Server.DataDir,
		"static_dir", cfg.Server.StaticDir,
		"sqlite_path", cfg.Storage.SQLite.Path,
		"timezone", time.Loc.String(),
		"mail_poll_secs", cfg.Server.MailPollSecs,
		"secret_key_set", cfg.Server.SecretKey != nil,
		"allow_insecure_http", cfg.Server.AllowInsecureHTTP,
		"log_level", cfg.Server.LogLevel,
		"log_file", cfg.Server.LogFile,
	)

	if err := os.MkdirAll(cfg.Server.DataDir, 0o755); err != nil {
		log.Error("creating data dir", "err", err)
		os.Exit(1)
	}
	avatarDir := cfg.Server.DataDir + "/avatars"
	if err := os.MkdirAll(avatarDir, 0o755); err != nil {
		log.Error("creating avatar dir", "err", err)
		os.Exit(1)
	}

	dbs, err := db.Open(cfg.Storage.SQLite.Path)
	if err != nil {
		log.Error("opening database", "err", err)
		os.Exit(1)
	}
	defer dbs.Write.Close()
	defer dbs.Read.Close()

	jwt := auth.New(cfg.Server.JWTSecret)
	userRepo := sqlite.NewUserRepo(dbs.Write)
	accountRepo := sqlite.NewAccountRepo(dbs.Write)
	categoryRepo := sqlite.NewCategoryRepo(dbs.Write)
	txnRepo := sqlite.NewTransactionRepo(dbs.Write)
	ruleRepo := sqlite.NewRuleRepo(dbs.Write)
	mailRepo := sqlite.NewMailRepo(dbs.Write)
	keyRepo := sqlite.NewUserKeyRepo(dbs.Write)

	authSvc := authservice.New(userRepo, jwt)
	userSvc := userservice.New(userRepo, jwt, avatarDir)
	accountSvc := accountservice.New(accountRepo)
	categorySvc := categoryservice.New(categoryRepo)

	transferSvc := transferservice.New(txnRepo, accountRepo)
	ruleSvc := ruleservice.New(ruleRepo, categoryRepo, txnRepo)
	transferRuleSvc := ruleservice.NewTransferRuleService(ruleRepo, txnRepo, accountRepo, transferSvc)
	txnSvc := transactionservice.New(txnRepo, accountRepo, categoryRepo).
		WithRule(ruleSvc).
		WithTransferRule(transferRuleSvc)

	mailSvc := mailservice.New(mailRepo, cfg.Server.SecretKey, txnSvc, accountSvc)
	mailSvc.Spawn(stdtime.Duration(cfg.Server.MailPollSecs) * stdtime.Second)
	keySvc := keyservice.New(keyRepo)

	api := server.New(server.Deps{
		Auth: authSvc, User: userSvc, Account: accountSvc, Category: categorySvc,
		Transaction: txnSvc, Transfer: transferSvc, Rule: ruleSvc,
		TransferRule: transferRuleSvc, Mail: mailSvc, Keys: keySvc,
		JWTSecret: cfg.Server.JWTSecret, StaticDir: cfg.Server.StaticDir,
		AvatarDir: avatarDir, DomainURL: cfg.Server.DomainURL,
		AllowInsecure: cfg.Server.AllowInsecureHTTP,
	})

	log.Info("server listening", "addr", cfg.Server.Addr)
	if err := http.ListenAndServe(cfg.Server.Addr, api.Handler()); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
