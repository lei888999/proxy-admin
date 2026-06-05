package main

import (
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
	inbHandler := handlers.NewInboundHandler(inbSvc, sbSvc)

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
		authed.DELETE("/inbounds/:id", inbHandler.DeleteInbound)
		authed.GET("/inbounds/:id/users", inbHandler.ListUsers)
		authed.POST("/inbounds/:id/users", inbHandler.CreateUser)
		authed.DELETE("/users/:id", inbHandler.DeleteUser)
	}

	web.Register(r)

	log.Printf("listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
