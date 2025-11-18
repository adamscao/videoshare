package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/adamscao/videoshare/internal/config"
	"github.com/adamscao/videoshare/internal/database"
	"github.com/adamscao/videoshare/internal/handler"
	"github.com/adamscao/videoshare/internal/middleware"
	"github.com/adamscao/videoshare/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	// Command line flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	initAdmin := flag.Bool("init-admin", false, "Initialize admin user")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	if err := database.InitDB(cfg.Database.Path); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create required directories
	if err := createRequiredDirectories(cfg); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	// Initialize sessions
	middleware.InitSessions(cfg)

	// Initialize admin if requested
	if *initAdmin {
		initializeAdmin()
		return
	}

	// Initialize services
	hlsService := service.NewHLSService(cfg)
	videoService := service.NewVideoService(cfg, hlsService)
	importService := service.NewImportService(cfg, videoService)
	authService := service.NewAuthService()

	// Initialize handlers
	authHandler := handler.NewAuthHandler(authService)
	uploadHandler := handler.NewUploadHandler(cfg, videoService)
	videoHandler := handler.NewVideoHandler(videoService)
	adminHandler := handler.NewAdminHandler(videoService, importService)

	// Setup router
	r := setupRouter(authHandler, uploadHandler, videoHandler, adminHandler)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func setupRouter(
	authHandler *handler.AuthHandler,
	uploadHandler *handler.UploadHandler,
	videoHandler *handler.VideoHandler,
	adminHandler *handler.AdminHandler,
) *gin.Engine {
	r := gin.Default()

	// Load templates
	r.LoadHTMLGlob("web/templates/**/*")

	// Serve static files
	r.Static("/static", "./web/static")

	// Public routes
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/upload")
	})

	// Upload routes
	r.GET("/upload", uploadHandler.ShowUploadPage)
	r.POST("/api/upload", uploadHandler.UploadVideo)

	// Video watch routes
	r.GET("/v/:slug", videoHandler.ShowVideoPage)
	r.GET("/api/video/:slug/info", videoHandler.GetVideoInfo)
	r.POST("/api/video/:slug/verify", videoHandler.VerifyVideoPassword)

	// HLS streaming routes
	r.GET("/hls/:slug/playlist.m3u8", videoHandler.ServeHLSPlaylist)
	r.GET("/hls/:slug/:segment", videoHandler.ServeHLSSegment)

	// Admin auth routes
	r.GET("/admin/login", authHandler.ShowLoginPage)
	r.POST("/api/admin/login", authHandler.Login)
	r.POST("/api/admin/logout", authHandler.Logout)

	// Admin protected routes
	admin := r.Group("/admin")
	admin.Use(middleware.AuthRequired())
	{
		admin.GET("/dashboard", adminHandler.ShowDashboard)
	}

	adminAPI := r.Group("/api/admin")
	adminAPI.Use(middleware.AuthRequired())
	{
		adminAPI.GET("/videos", adminHandler.GetVideos)
		adminAPI.GET("/videos/:id", adminHandler.GetVideo)
		adminAPI.PUT("/videos/:id", adminHandler.UpdateVideo)
		adminAPI.DELETE("/videos/:id", adminHandler.DeleteVideo)
		adminAPI.POST("/import", adminHandler.ImportVideos)
		adminAPI.GET("/settings", adminHandler.GetSettings)
		adminAPI.PUT("/settings", adminHandler.UpdateSettings)
	}

	return r
}

func initializeAdmin() {
	var username, password string

	fmt.Print("Enter admin username: ")
	fmt.Scanln(&username)

	fmt.Print("Enter admin password: ")
	fmt.Scanln(&password)

	if username == "" || password == "" {
		log.Fatal("Username and password cannot be empty")
	}

	authService := service.NewAuthService()
	if err := authService.CreateAdmin(username, password); err != nil {
		log.Fatalf("Failed to create admin: %v", err)
	}

	fmt.Println("Admin user created successfully!")
}

func createRequiredDirectories(cfg *config.Config) error {
	dirs := []string{
		cfg.Storage.VideosDir,
		cfg.Storage.OriginalsDir,
		cfg.Storage.HLSDir,
		cfg.Storage.ImportDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}
