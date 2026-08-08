package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"github.com/quic-go/quic-go/http3"

	"github.com/mohan9182/financer/db"
	"github.com/mohan9182/financer/httpserver"
	"github.com/mohan9182/financer/repository"
	"github.com/mohan9182/financer/services"
)

func main() {
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
	log.Println("Database migrations applied successfully")

	authSvc := services.NewAuthService(writeDB, jwtSecret)

	baseRepo := repository.NewBaseRepository(writeDB, readDB)
	accountRepo := repository.NewAccountRepository(baseRepo)
	txnRepo := repository.NewTransactionRepository(baseRepo)
	transferRepo := repository.NewTransferRepository(baseRepo)
	accountSvc := services.NewAccountService(accountRepo)
	transactionSvc := services.NewTransactionService(txnRepo, accountRepo)
	transferSvc := services.NewTransferService(txnRepo, transferRepo, accountRepo)

	api := httpserver.NewAPI(authSvc, accountSvc, transactionSvc, transferSvc, jwtSecret)
	handler := api.Handler()

	// HTTP/1.1 + HTTP/2 for the browser (dev uses this via fetch on localhost).
	log.Printf("Financer JSON server (HTTP/1.1+2) listening on http://localhost%s", addr)
	go func() {
		if err := http.ListenAndServe(addr, handler); err != nil {
			log.Fatalf("Failed to start TCP server: %v", err)
		}
	}()

	// HTTP/3 (QUIC). Needs a cert; browsers only engage HTTP/3 over HTTPS on
	// a real domain. Optional — omitted unless a cert is provided.
	certFile, keyFile := os.Getenv("FINANCER_TLS_CERT"), os.Getenv("FINANCER_TLS_KEY")
	if certFile == "" || keyFile == "" {
		log.Println("HTTP/3 skipped: set FINANCER_TLS_CERT + FINANCER_TLS_KEY to enable")
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
	log.Printf("HTTP/3 (QUIC) server listening on https://localhost%s", addr)
	if err := h3.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start HTTP/3 server: %v", err)
	}
}
