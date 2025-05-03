package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gomodules.avm99963.com/zenithplanner/internal/calendar"
	"gomodules.avm99963.com/zenithplanner/internal/config"
	"gomodules.avm99963.com/zenithplanner/internal/database"
	"gomodules.avm99963.com/zenithplanner/internal/handler"
	"gomodules.avm99963.com/zenithplanner/internal/scheduler"
	"gomodules.avm99963.com/zenithplanner/internal/sync"
)

func main() {
	log.Println("Starting ZenithPlanner Backend Service...")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Printf("Received signal: %s, initiating shutdown...", sig)
		cancel()
	}()

	cfg := loadConfiguration()

	dbPool, err := database.NewDBPool(ctx, cfg.DB)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()
	log.Println("Database connection pool initialized.")
	dbRepo := database.NewRepository(dbPool)

	calendarService, err := calendar.NewService(ctx, cfg.Google)
	if err != nil {
		log.Fatalf("Failed to create Calendar client: %v", err)
	}
	log.Println("Google Calendar client initialized.")

	syncer := sync.NewSyncer(dbRepo, calendarService, cfg)

	if cfg.App.EnableCalendarSubscription {
		err := syncer.EnsureWebhookChannelExists(ctx)
		if err != nil {
			log.Fatalf("Failed to ensure webhook channel exists on startup: %v", err)
		}
	}

	runInitialSync(syncer)

	taskScheduler := scheduler.NewScheduler(syncer, &cfg.App)
	taskScheduler.Start()

	syncer.StartSyncWorker(ctx)

	httpServer := startHttpServer(syncer, cfg)

	log.Println("ZenithPlanner Backend Service - Initialization complete. Running...")

	// Wait for shutdown signal
	<-ctx.Done()

	log.Println("Shutting down ZenithPlanner backend service...")

	schedulerCtx := taskScheduler.Stop()
	<-schedulerCtx.Done()
	log.Println("Scheduler stopped.")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server Shutdown error: %v", err)
	} else {
		log.Println("HTTP server gracefully stopped.")
	}

	log.Println("Shutdown complete.")
}

func loadConfiguration() *config.Config {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("Configuration loaded.\n")
	return cfg
}

func runInitialSync(syncer *sync.Syncer) {
	go func() {
		log.Println("Requesting initial sync on startup...")
		syncer.RequestSync()
	}()
}

func startHttpServer(syncer *sync.Syncer, cfg *config.Config) *http.Server {
	webhookHandler := handler.NewWebhookHandler(syncer, cfg)
	mux := http.NewServeMux()
	handler.RegisterWebhookRoute(mux, webhookHandler)

	serverAddr := ":8080"
	httpServer := &http.Server{
		Addr:    serverAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("Starting HTTP server, listening for webhooks on %s", serverAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe error: %v", err)
		}
		log.Println("HTTP server stopped.")
	}()

	return httpServer
}
