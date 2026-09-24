package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/auth"
	"singbox-admin/internal/config"
	"singbox-admin/internal/database"
	"singbox-admin/internal/handlers"
	"singbox-admin/internal/inbound"
	"singbox-admin/internal/middleware"
	"singbox-admin/internal/singbox"
	"singbox-admin/internal/traffic"
	"singbox-admin/internal/web"
)

// maxAPIBody caps request bodies. The config endpoint accepts arbitrary JSON, so
// without a ceiling a single request can push hundreds of MB through memory and
// onto disk.
const maxAPIBody = 1 << 20 // 1 MiB

// Server timeouts. Go's defaults are "no timeout", which leaves a panel exposed
// on a public port open to connections that never finish.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownGrace     = 10 * time.Second
)

func main() {
	cfg := config.Load()

	db, err := database.Init(cfg.DBPath, cfg.DefaultAdminUser, cfg.DefaultAdminPass)
	if err != nil {
		log.Fatalf("database init: %v", err)
	}

	jm := auth.NewJWTManager(cfg.JWTSecret, 7*24*time.Hour)
	authHandler := handlers.NewAuthHandler(db, jm, cfg.CookieSecure)
	if !cfg.CookieSecure {
		log.Println("WARN: COOKIE_SECURE is off — the session cookie is sent over plain HTTP. Enable it behind TLS.")
	}

	// The singbox dir holds config.json (0600) plus the log and pid files; keep
	// the directory itself owner-only so nothing inside it is browsable.
	if err := os.MkdirAll(cfg.SingboxDir, 0700); err != nil {
		log.Printf("WARN: could not create %s: %v", cfg.SingboxDir, err)
	} else if err := os.Chmod(cfg.SingboxDir, 0700); err != nil {
		log.Printf("WARN: could not restrict permissions on %s: %v", cfg.SingboxDir, err)
	}

	sbSvc := singbox.NewDefault(cfg.SingboxDir, cfg.SingboxBin)
	sbHandler := handlers.NewSingboxHandler(sbSvc)

	inbSvc := inbound.NewService(db, sbSvc)

	// Claim the ports the panel itself listens on, so an inbound cannot be
	// created on one. sing-box would just fail to bind, and the only trace would
	// be in its own log.
	if p, err := strconv.ParseUint(cfg.Port, 10, 16); err == nil {
		inbSvc.ReservePort(uint16(p), "tcp", "面板端口")
	}

	exp, err := inbSvc.APIConfig()
	if err != nil {
		log.Fatalf("api config: %v", err)
	}
	if p, ok := portOf(exp.ClashAddr); ok {
		inbSvc.ReservePort(p, "tcp", "sing-box Clash API 端口")
	}

	// Seeding and backfill regenerate the config, which can fail (no sing-box
	// binary yet, a bad stored inbound). That must not stop the panel from
	// starting: exiting here leaves no way to log in and fix it.
	if err := inbound.SeedDefaults(inbSvc); err != nil {
		log.Printf("WARN: seed defaults: %v", err)
	}
	if err := inbSvc.BackfillUserTokens(); err != nil {
		log.Printf("WARN: backfill tokens: %v", err)
	}
	startManagedSingbox(sbSvc)

	clashClient := traffic.NewClashClient(exp.ClashAddr, exp.ClashSecret)
	// Per-user usage comes from the active-connections snapshot API. Poll once a
	// second to reduce the blind window for short connections; the poller batches
	// DB writes separately, so this does not turn into a write per user per tick.
	poller := traffic.NewPoller(clashClient, inbSvc, time.Second, sbSvc.ManagedRunning)
	liveMonitor := traffic.NewLiveMonitor(exp.ClashAddr, exp.ClashSecret, sbSvc.ManagedRunning)
	pollCtx, stopPoller := context.WithCancel(context.Background())
	defer stopPoller()
	go poller.Run(pollCtx)
	go liveMonitor.Run(pollCtx)

	inbHandler := handlers.NewInboundHandler(inbSvc, sbSvc)
	userHandler := handlers.NewUserHandler(inbSvc)
	outboundHandler := handlers.NewOutboundHandler(inbSvc)
	subHandler := handlers.NewSubscriptionHandler(inbSvc, cfg.ServerHost)
	trafficHandler := handlers.NewTrafficHandler(liveMonitor)
	diagnosticsHandler := handlers.NewDiagnosticsHandler(inbSvc)

	r := gin.Default()
	// gin trusts every proxy by default, which makes X-Forwarded-For — and so the
	// client IP the login throttle keys on — trivially spoofable.
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatalf("trusted proxies: %v", err)
	}

	api := r.Group("/api", middleware.MaxBody(maxAPIBody))
	{
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/logout", authHandler.Logout)

		authed := api.Group("", middleware.RequireAuth(jm, handlers.SessionValid(db)))
		authed.PUT("/auth/password", authHandler.ChangePassword)

		authed.GET("/status", sbHandler.Status)
		authed.GET("/singbox/config", sbHandler.GetConfig)
		authed.PUT("/singbox/config", sbHandler.PutConfig)
		authed.POST("/singbox/start", sbHandler.Start)
		authed.POST("/singbox/stop", sbHandler.Stop)
		authed.POST("/singbox/apply", inbHandler.Apply)

		authed.GET("/inbound-types", inbHandler.ListTypes)
		authed.GET("/inbounds", inbHandler.ListInbounds)
		authed.POST("/inbounds", inbHandler.CreateInbound)
		authed.PUT("/inbounds/:id", inbHandler.UpdateInbound)
		authed.POST("/inbounds/:id/reset-keys", inbHandler.ResetKeys)
		authed.DELETE("/inbounds/:id", inbHandler.DeleteInbound)

		authed.GET("/users", userHandler.List)
		authed.POST("/users", userHandler.Create)
		authed.PUT("/users/:id", userHandler.Update)
		authed.POST("/users/:id/reset", userHandler.Reset)
		authed.POST("/users/:id/reset-traffic", userHandler.ResetTraffic)
		authed.DELETE("/users/:id", userHandler.Delete)

		authed.GET("/outbounds", outboundHandler.List)
		authed.POST("/outbounds", outboundHandler.Create)
		authed.PUT("/outbounds/:id", outboundHandler.Update)
		authed.DELETE("/outbounds/:id", outboundHandler.Delete)

		authed.GET("/traffic/live", trafficHandler.Live)
		authed.GET("/traffic/diagnostics/outbounds", diagnosticsHandler.Outbounds)
	}

	r.GET("/sub/:token", subHandler.Get)

	web.Register(r)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	// Drain in-flight requests on shutdown. A request killed mid-ApplyConfig is
	// how the config file, the pid file and the live process end up disagreeing,
	// and systemd's Restart= then runs straight back into that state.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down")
	stopPoller()
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

// startupSingbox is the narrow part of singbox.Service used at process start.
// Keep it small and injectable so startup policy can be tested without a real
// binary, PID file or system process.
type startupSingbox interface {
	Status() singbox.Status
	Start() (singbox.Status, error)
}

// startManagedSingbox starts a configured sing-box whenever the panel starts.
// A failure must not terminate the admin panel: users need the UI to read the
// detailed state/log and correct a bad config or a port conflict. ProcessManager
// still rejects duplicate panel-owned starts and never touches external ones.
func startManagedSingbox(s startupSingbox) {
	st := s.Status()
	if !st.Installed || !st.HasConfig || st.Running {
		return
	}
	if _, err := s.Start(); err != nil {
		log.Printf("WARN: auto-start sing-box: %v", err)
		return
	}
	log.Println("started panel-managed sing-box")
}

// portOf extracts the port from a host:port authority.
func portOf(addr string) (uint16, bool) {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, false
	}
	n, err := strconv.ParseUint(p, 10, 16)
	if err != nil {
		return 0, false
	}
	return uint16(n), true
}
