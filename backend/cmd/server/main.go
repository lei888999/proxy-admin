package main

import (
	"context"
	"log"
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

func main() {
	cfg := config.Load()

	db, err := database.Init(cfg.DBPath, cfg.DefaultAdminUser, cfg.DefaultAdminPass)
	if err != nil {
		log.Fatalf("database init: %v", err)
	}

	jm := auth.NewJWTManager(cfg.JWTSecret, 7*24*time.Hour)
	authHandler := handlers.NewAuthHandler(db, jm)

	sbSvc := singbox.NewDefault(cfg.SingboxDir, cfg.SingboxBin)
	sbHandler := handlers.NewSingboxHandler(sbSvc)

	inbSvc := inbound.NewService(db, sbSvc)
	if err := inbound.SeedDefaults(inbSvc); err != nil {
		log.Fatalf("seed defaults: %v", err)
	}
	if err := inbSvc.BackfillUserTokens(); err != nil {
		log.Fatalf("backfill tokens: %v", err)
	}

	exp, err := inbSvc.APIConfig()
	if err != nil {
		log.Fatalf("api config: %v", err)
	}
	clashClient := traffic.NewClashClient(exp.ClashAddr, exp.ClashSecret)
	poller := traffic.NewPoller(clashClient, inbSvc, 10*time.Second)
	go poller.Run(context.Background())

	inbHandler := handlers.NewInboundHandler(inbSvc, sbSvc)
	userHandler := handlers.NewUserHandler(inbSvc)
	subHandler := handlers.NewSubscriptionHandler(inbSvc, cfg.ServerHost)
	trafficHandler := handlers.NewTrafficHandler(exp.ClashAddr, exp.ClashSecret)

	r := gin.Default()

	api := r.Group("/api")
	{
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/logout", authHandler.Logout)

		authed := api.Group("", middleware.RequireAuth(jm))
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

		authed.GET("/traffic/live", trafficHandler.Live)
	}

	r.GET("/sub/:token", subHandler.Get)

	web.Register(r)

	log.Printf("listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
