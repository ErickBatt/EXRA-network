package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"exra/config"
	"exra/db"
	"exra/gwclaims"
	"exra/handlers"
	"exra/hub"
	"exra/middleware"
	"exra/models"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	cfg := config.LoadConfig()

	// Fail fast if no Gateway-JWT signing key is configured. There is no
	// hardcoded fallback anymore (AUDIT §1 D1).
	gwclaims.MustInitSigner()
	db.Init(cfg.SupabaseURL)
	handlers.SetRatePerGB(cfg.RatePerGB)
	maxSupply, _ := strconv.ParseFloat(cfg.ExraMaxSupply, 64)
	epochSize, _ := strconv.ParseFloat(cfg.ExraEpochSize, 64)
	_ = maxSupply
	_ = epochSize
	// Wire PoP emission from env into the reward engine.
	if e, err := strconv.ParseFloat(cfg.PopEmissionPerHeartbeat, 64); err == nil {
		models.SetPopEmission(e)
	}
	wsHub := hub.NewHub()
	wsHub.InitRedis(cfg.RedisURL)
	wsHub.OnOracleProposal = func(prop models.OracleProposal) {
		models.ProcessOracleProposal(prop, cfg.OracleNodes)
	}
	models.InitPeaq()
	hub.InitGeo("GeoLite2-City.mmdb")
	handlers.SetHub(wsHub)
	models.SetHub(wsHub)
	go wsHub.Run()

	// ── 3.5 Start High-Performance Control Plane (Fiber) ──
	controlPort := os.Getenv("CONTROL_PORT")
	if controlPort == "" {
		controlPort = "8081"
	}
	StartControlPlane(controlPort, wsHub)

	// ── 4. Start Daily Oracle Batch Worker & PoP Scaling Worker ──
	models.StartPopWorker()
	go models.RunOracleWorker(cfg.OracleNodes, wsHub.BroadcastOracleProposal)

	nodeAuth := middleware.DIDAuth
	proxyAuth := middleware.NodeAuth(cfg.ProxySecret) // admin secret for creating buyers
	adminOps := middleware.AdminAuth("admin_ops", "admin_finance", "admin_readonly", "admin_root")
	adminMutating := middleware.AdminAuth("admin_ops", "admin_finance", "admin_root")
	adminFinance := middleware.AdminAuth("admin_finance", "admin_root")

	r := mux.NewRouter()
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS)
	r.Use(middleware.LimitRequestBody)
	r.Use(middleware.RequestLogger)
	r.Use(middleware.RateLimit)

	// Health check (public)
	r.HandleFunc("/health", handlers.HealthCheck).Methods("GET")

	// Prometheus metrics — protected; exposes internal topology data
	r.Handle("/metrics", proxyAuth(promhttp.Handler().ServeHTTP)).Methods("GET")

	// Node API (authenticated with NODE_SECRET)
	r.HandleFunc("/api/node/register", nodeAuth(handlers.RegisterNode)).Methods("POST")
	r.HandleFunc("/api/node/{id}/heartbeat", nodeAuth(handlers.NodeHeartbeat)).Methods("POST")
	r.HandleFunc("/api/node/tunnel", nodeAuth(handlers.TunnelHandler)).Methods("GET")
	r.HandleFunc("/api/node/set-referrer", nodeAuth(handlers.SetReferrerHandler)).Methods("POST")
	r.HandleFunc("/api/nodes", handlers.ListNodes).Methods("GET")
	r.HandleFunc("/api/nodes/market-price", handlers.GetMarketPrice).Methods("GET")
	r.HandleFunc("/api/node/earnings", proxyAuth(handlers.GetNodeEarnings)).Methods("GET")
	r.HandleFunc("/ws", handlers.WsHandler(wsHub)).Methods("GET")
	r.HandleFunc("/ws/map", handlers.LiveMapHandler(wsHub)).Methods("GET")
	r.HandleFunc("/nodes", handlers.PublicNodes).Methods("GET")
	r.HandleFunc("/nodes/stats", handlers.PublicNodeStats).Methods("GET")

	// Admin: create buyer (authenticated with PROXY_SECRET)
	r.HandleFunc("/api/buyer/register", proxyAuth(handlers.RegisterBuyer)).Methods("POST")
	// Deprecated: buyer sync is now admin-only to prevent API key leakage by email.
	r.HandleFunc("/api/buyer/sync", adminOps(handlers.SyncBuyer)).Methods("POST")
	r.HandleFunc("/api/payout/precheck", proxyAuth(handlers.PrecheckPayout)).Methods("POST")
	r.HandleFunc("/api/payout/request", proxyAuth(handlers.RequestPayout)).Methods("POST")
	r.HandleFunc("/api/payout/{id}/approve", proxyAuth(handlers.ApprovePayout)).Methods("POST")
	r.HandleFunc("/api/payouts", proxyAuth(handlers.ListPayouts)).Methods("GET")
	
	// Payout v2
	r.HandleFunc("/claim/{did}", handlers.ClaimPayoutHandler).Methods("POST")
	
	r.HandleFunc("/api/tokenomics/oracle/process", proxyAuth(handlers.ProcessOracleQueue)).Methods("POST")
	r.HandleFunc("/api/tokenomics/oracle/queue", proxyAuth(handlers.GetOracleQueue)).Methods("GET")
	r.HandleFunc("/api/tokenomics/oracle/queue/{id}/retry", proxyAuth(handlers.RetryOracleQueueItem)).Methods("POST")
	r.HandleFunc("/api/tokenomics/payments/settle", proxyAuth(handlers.SettleBuyerPayment)).Methods("POST")
	r.HandleFunc("/api/tokenomics/stats", proxyAuth(handlers.GetTokenomicsStats)).Methods("GET")
	r.HandleFunc("/api/tokenomics/epoch", handlers.GetEpochState).Methods("GET") // public: FOMO counter
	r.HandleFunc("/api/audit/mints", handlers.PublicMintAudit).Methods("GET")    // public: mint transparency log
	r.HandleFunc("/api/tokenomics/swap/quote", nodeAuth(handlers.RequestSwapQuote)).Methods("POST")
	r.HandleFunc("/api/tokenomics/swap/execute", nodeAuth(handlers.ExecuteSwap)).Methods("POST")
	r.HandleFunc("/api/test/proxy-task", proxyAuth(handlers.DispatchProxyTask)).Methods("POST")

	// Admin API v1 (role-bound auth + audit-ready actions)
	r.HandleFunc("/api/admin/tokenomics/stats", adminOps(handlers.AdminTokenomicsStats)).Methods("GET")
	r.HandleFunc("/api/admin/oracle/queue", adminOps(handlers.AdminOracleQueue)).Methods("GET")
	r.HandleFunc("/api/admin/oracle/queue/{id}/retry", adminMutating(handlers.AdminRetryOracleQueueItem)).Methods("POST")
	r.HandleFunc("/api/admin/oracle/process", adminMutating(handlers.AdminProcessOracleQueue)).Methods("POST")
	r.HandleFunc("/api/admin/peaq/trigger-batch", adminMutating(handlers.AdminTriggerPeaqBatch)).Methods("POST")
	r.HandleFunc("/api/admin/payouts", adminOps(handlers.AdminListPayouts)).Methods("GET")
	r.HandleFunc("/api/admin/payout/{id}/approve", adminFinance(handlers.AdminApprovePayout)).Methods("POST")
	r.HandleFunc("/api/admin/payout/{id}/reject", adminFinance(handlers.AdminRejectPayout)).Methods("POST")
	r.HandleFunc("/api/admin/incidents", adminOps(handlers.AdminIncidents)).Methods("GET")
	r.HandleFunc("/api/admin/node/freeze", adminMutating(handlers.AdminFreezeNode)).Methods("POST")
	r.HandleFunc("/api/admin/circuit-breaker", adminOps(handlers.AdminCircuitBreakerState)).Methods("GET")

	// Buyer API (authenticated with buyer's API key)
	r.HandleFunc("/api/buyer/me", middleware.BuyerAuth(handlers.GetBuyerProfile)).Methods("GET")
	r.HandleFunc("/api/buyer/sessions", middleware.BuyerAuth(handlers.GetBuyerSessions)).Methods("GET")
	r.HandleFunc("/api/buyer/topup", middleware.BuyerAuth(handlers.TopUpBalance)).Methods("POST")

	// Session management
	r.HandleFunc("/api/session/start", middleware.BuyerAuth(handlers.StartSession)).Methods("POST")
	r.HandleFunc("/api/session/{id}/end", middleware.BuyerAuth(handlers.EndSession)).Methods("POST")
	r.HandleFunc("/api/offers", middleware.BuyerAuth(handlers.CreateOffer)).Methods("POST")
	r.HandleFunc("/api/offers", middleware.BuyerAuth(handlers.ListOffers)).Methods("GET")
	r.HandleFunc("/api/offers/{id}/assign", middleware.BuyerAuth(handlers.AssignOffer)).Methods("POST")

	// Compute Market
	r.HandleFunc("/api/compute/submit", middleware.BuyerAuth(handlers.SubmitTask)).Methods("POST")
	r.HandleFunc("/api/compute/jobs/{id}", middleware.BuyerAuth(handlers.GetTaskStatus)).Methods("GET")
	r.HandleFunc("/api/compute/node/result", nodeAuth(handlers.SubmitComputeResult)).Methods("POST")
	
	// TMA Extra
	r.HandleFunc("/api/tma/stake", nodeAuth(handlers.TmaStake)).Methods("POST")

	// Pool system (Phase 3) — node auth required for mutations
	r.HandleFunc("/api/pools", handlers.ListPools).Methods("GET")                       // public
	r.HandleFunc("/api/pools", nodeAuth(handlers.CreatePool)).Methods("POST")
	r.HandleFunc("/api/pools/{id}", handlers.GetPool).Methods("GET")                    // public
	r.HandleFunc("/api/pools/{id}/join", nodeAuth(handlers.JoinPool)).Methods("POST")
	r.HandleFunc("/api/pools/leave", nodeAuth(handlers.LeavePool)).Methods("POST")
	r.HandleFunc("/api/pools/me", nodeAuth(handlers.GetMyPool)).Methods("GET")

	// Telegram Mini App API
	r.HandleFunc("/api/tma/auth", handlers.TmaAuth).Methods("POST")                                    // public — Telegram initData auth
	r.HandleFunc("/api/tma/link-device", handlers.TmaLinkDevice).Methods("POST")                       // public — link device to TG account
	r.HandleFunc("/api/tma/me", nodeAuth(handlers.TmaMe)).Methods("GET")                               // node auth
	r.HandleFunc("/api/tma/earnings", nodeAuth(handlers.TmaEarnings)).Methods("GET")                   // node auth
	r.HandleFunc("/api/tma/withdraw", nodeAuth(handlers.TmaWithdraw)).Methods("POST")                  // node auth
	r.HandleFunc("/api/tma/epoch", handlers.TmaEpoch).Methods("GET")                                   // public
	r.HandleFunc("/api/tma/push-token", nodeAuth(handlers.TmaRegisterPushToken)).Methods("POST")       // node auth

	// HTTP CONNECT proxy endpoint
	r.HandleFunc("/proxy", middleware.BuyerAuth(handlers.HTTPConnectProxy)).Methods("POST", "CONNECT", "GET")

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("Exra server starting on :%s (rate: $%s/GB)", cfg.Port, cfg.RatePerGB)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
	}
}
