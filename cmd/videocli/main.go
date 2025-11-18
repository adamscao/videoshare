package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/adamscao/videoshare/internal/config"
	"github.com/adamscao/videoshare/internal/database"
	"github.com/adamscao/videoshare/internal/models"
	"github.com/adamscao/videoshare/internal/utils"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	action := flag.String("action", "", "Action: list, set-password, set-public, set-private")
	slug := flag.String("slug", "", "Video slug")
	password := flag.String("password", "", "Video password")
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

	switch *action {
	case "list":
		listVideos()
	case "set-password":
		if *slug == "" || *password == "" {
			log.Fatal("Both --slug and --password are required")
		}
		setVideoPassword(*slug, *password)
	case "set-public":
		if *slug == "" {
			log.Fatal("--slug is required")
		}
		setVideoPublic(*slug, true)
	case "set-private":
		if *slug == "" {
			log.Fatal("--slug is required")
		}
		setVideoPublic(*slug, false)
	default:
		fmt.Println("Usage:")
		fmt.Println("  List all videos:")
		fmt.Println("    ./videocli --action list")
		fmt.Println("  Set video password:")
		fmt.Println("    ./videocli --action set-password --slug VIDEO_SLUG --password PASSWORD")
		fmt.Println("  Make video public (remove password):")
		fmt.Println("    ./videocli --action set-public --slug VIDEO_SLUG")
		fmt.Println("  Make video private (require password):")
		fmt.Println("    ./videocli --action set-private --slug VIDEO_SLUG --password PASSWORD")
		os.Exit(1)
	}
}

func listVideos() {
	var videos []models.Video
	database.DB.Order("created_at DESC").Find(&videos)

	fmt.Printf("%-15s %-40s %-15s %-10s\n", "SLUG", "TITLE", "CREATED", "PROTECTED")
	fmt.Println(strings.Repeat("-", 85))

	for _, v := range videos {
		protected := "No"
		if v.IsPasswordProtected {
			protected = "Yes"
		}
		fmt.Printf("%-15s %-40s %-15s %-10s\n",
			v.Slug,
			truncate(v.Title, 38),
			v.CreatedAt.Format("2006-01-02"),
			protected,
		)
	}
}

func setVideoPassword(slug, password string) {
	var video models.Video
	if err := database.DB.Where("slug = ?", slug).First(&video).Error; err != nil {
		log.Fatalf("Video not found: %v", err)
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	updates := map[string]interface{}{
		"is_password_protected": true,
		"password_hash":         hash,
	}

	if err := database.DB.Model(&video).Updates(updates).Error; err != nil {
		log.Fatalf("Failed to update video: %v", err)
	}

	fmt.Printf("✓ Password set for video: %s\n", video.Title)
}

func setVideoPublic(slug string, isPublic bool) {
	var video models.Video
	if err := database.DB.Where("slug = ?", slug).First(&video).Error; err != nil {
		log.Fatalf("Video not found: %v", err)
	}

	updates := map[string]interface{}{
		"is_password_protected": !isPublic,
	}

	if isPublic {
		updates["password_hash"] = ""
	}

	if err := database.DB.Model(&video).Updates(updates).Error; err != nil {
		log.Fatalf("Failed to update video: %v", err)
	}

	status := "public"
	if !isPublic {
		status = "private (password required)"
	}
	fmt.Printf("✓ Video '%s' is now %s\n", video.Title, status)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
