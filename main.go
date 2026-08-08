package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/quic-go/quic-go/http3"

	"github.com/mohan9182/financer/db"
	"github.com/mohan9182/financer/httpserver"
	"github.com/mohan9182/financer/logx"
	"github.com/mohan9182/financer/repository"
	"github.com/mohan9182/financer/services"
)

func main() {
	logx.Init()

	dbPath := os.Getenv("FINANCER_DB_PATH")
	if dbPath == "" {
		dbPath = "data/financer.db"
	}
	jwtSecret := os.Getenv("FINANCER_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-in-production"
	}
	addr := os.Getenv("FINANCER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	writeDB, readDB, err := db.OpenDBs(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer writeDB.Close()
	defer readDB.Close()

	if err := db.RunMigrations(writeDB); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	logx.Info("database migrations applied successfully")

	authSvc := services.NewAuthService(writeDB, jwtSecret)

	avatarDir := filepath.Join(filepath.Dir(dbPath), "avatars")
	if err := os.MkdirAll(avatarDir, 0o755); err != nil {
		log.Fatalf("Failed to create avatar dir: %v", err)
	}
	userSvc := services.NewUserService(writeDB, jwtSecret, avatarDir)

	baseRepo := repository.NewBaseRepository(writeDB, readDB)
	accountRepo := repository.NewAccountRepository(baseRepo)
	txnRepo := repository.NewTransactionRepository(baseRepo)
	transferRepo := repository.NewTransferRepository(baseRepo)
	categoryRepo := repository.NewCategoryRepository(baseRepo)
	ruleRepo := repository.NewRuleRepository(baseRepo)
	accountSvc := services.NewAccountService(accountRepo)
	categorySvc := services.NewCategoryService(categoryRepo)
	ruleSvc := services.NewRuleService(ruleRepo, categoryRepo, txnRepo)
	transferRuleSvc := services.NewTransferRuleService(ruleRepo, txnRepo, transferRepo, accountRepo)
	transactionSvc := services.NewTransactionService(txnRepo, accountRepo, ruleSvc, transferRuleSvc)
	transferSvc := services.NewTransferService(txnRepo, transferRepo, accountRepo)
	investmentRepo := repository.NewInvestmentRepository(baseRepo)
	yahoo, err := services.NewYahooClient()
	if err != nil {
		log.Fatalf("Failed to load Yahoo config: %v", err)
	}
	investmentSvc := services.NewInvestmentService(investmentRepo, yahoo)

	api := httpserver.NewAPI(authSvc, userSvc, accountSvc, categorySvc, transactionSvc, transferSvc, ruleSvc, transferRuleSvc, investmentSvc, jwtSecret)

	mux := http.NewServeMux()
	mux.Handle("/avatars/", http.StripPrefix("/avatars/", http.FileServer(http.Dir(avatarDir))))
	mux.Handle("/", api.Handler())
	handler := mux

	// Background quote + price-history refreshers for all users' investments.
	runPeriodic("FINANCER_QUOTE_REFRESH_INTERVAL", 30*time.Minute, 60*time.Second, func(ctx context.Context) error {
		return investmentSvc.RefreshAllPrices(ctx)
	})
	runPeriodic("FINANCER_HISTORY_REFRESH_INTERVAL", 30*time.Minute, 5*time.Minute, func(ctx context.Context) error {
		return investmentSvc.RefreshAllPriceHistory(ctx)
	})

	// HTTP/1.1 + HTTP/2 for the browser (dev uses this via fetch on localhost).
	logx.Info("Financer JSON server listening", "addr", addr)
	go func() {
		if err := http.ListenAndServe(addr, handler); err != nil {
			log.Fatalf("Failed to start TCP server: %v", err)
		}
	}()

	// HTTP/3 (QUIC). Needs a cert; browsers only engage HTTP/3 over HTTPS on
	// a real domain. Optional — omitted unless a cert is provided.
	certFile, keyFile := os.Getenv("FINANCER_TLS_CERT"), os.Getenv("FINANCER_TLS_KEY")
	if certFile == "" || keyFile == "" {
		logx.Info("HTTP/3 skipped: set FINANCER_TLS_CERT + FINANCER_TLS_KEY to enable")
		select {}
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("Failed to load TLS cert: %v", err)
	}
	h3 := &http3.Server{
		Addr:    addr,
		Handler: handler,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}
	logx.Info("HTTP/3 (QUIC) server listening", "addr", addr)
	if err := h3.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start HTTP/3 server: %v", err)
	}
}

// runPeriodic runs fn on a ticker with interval from envName (fallback to
// def when unset), skipping the first tick so the app starts immediately.
func runPeriodic(envName string, def, min time.Duration, fn func(ctx context.Context) error) {
	interval := def
	if v := os.Getenv(envName); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= min {
			interval = d
		}
	}
	go func() {
		ticker := time.NewTicker(interval)
		logx.Info("refresher started", "name", envName, "interval", interval.String())
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), interval)
			if err := fn(ctx); err != nil {
				logx.Error("refresher failed", "name", envName, "err", err)
			}
			cancel()
		}
	}()
}
