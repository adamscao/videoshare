package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adamscao/videoshare/internal/config"
	"github.com/adamscao/videoshare/internal/database"
	"github.com/adamscao/videoshare/internal/models"
)

type ImportService struct {
	config       *config.Config
	videoService *VideoService
}

type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Failed   int      `json:"failed"`
	Messages []string `json:"messages"`
}

var supportedExtensions = []string{".mp4", ".avi", ".mkv", ".mov", ".flv", ".wmv"}

func NewImportService(cfg *config.Config, videoService *VideoService) *ImportService {
	return &ImportService{
		config:       cfg,
		videoService: videoService,
	}
}

// ScanAndImport scans import directory and imports new videos
func (s *ImportService) ScanAndImport() (*ImportResult, error) {
	result := &ImportResult{
		Messages: []string{},
	}

	// Read import directory
	entries, err := os.ReadDir(s.config.Storage.ImportDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read import dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		ext := strings.ToLower(filepath.Ext(filename))

		// Check if file type is supported
		if !isSupportedVideoFile(ext) {
			result.Messages = append(result.Messages, fmt.Sprintf("Skipped unsupported file: %s", filename))
			result.Skipped++
			continue
		}

		// Check if already imported (by filename)
		var existing models.Video
		if err := database.DB.Where("original_filename = ?", filename).First(&existing).Error; err == nil {
			result.Messages = append(result.Messages, fmt.Sprintf("Skipped already imported: %s", filename))
			result.Skipped++
			continue
		}

		// Import the video
		importPath := filepath.Join(s.config.Storage.ImportDir, filename)
		if err := s.importVideo(importPath, filename, result); err != nil {
			result.Messages = append(result.Messages, fmt.Sprintf("Failed to import %s: %v", filename, err))
			result.Failed++
		} else {
			result.Messages = append(result.Messages, fmt.Sprintf("Successfully imported: %s", filename))
			result.Imported++
		}
	}

	return result, nil
}

func (s *ImportService) importVideo(importPath, filename string, result *ImportResult) error {
	// Move file to originals directory
	destPath := filepath.Join(s.config.Storage.OriginalsDir, filename)

	// Copy file instead of move to preserve original
	if err := copyFile(importPath, destPath); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Generate title from filename (remove extension)
	title := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Create video record
	_, err := s.videoService.CreateVideo(
		destPath,
		filename,
		title,
		"", // empty description
		"import",
		false, // not password protected
		"",
	)

	if err != nil {
		// Clean up on error
		os.Remove(destPath)
		return err
	}

	// Optionally remove from import dir after successful import
	// os.Remove(importPath)

	return nil
}

func isSupportedVideoFile(ext string) bool {
	for _, supported := range supportedExtensions {
		if ext == supported {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	sourceFile, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, sourceFile, 0644)
}
