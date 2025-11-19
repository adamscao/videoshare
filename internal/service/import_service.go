package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adamscao/videoshare/internal/config"
	"github.com/adamscao/videoshare/internal/database"
	"github.com/adamscao/videoshare/internal/models"
)

type ImportService struct {
	config       *config.Config
	videoService *VideoService
	scanning     bool
	scanMutex    sync.Mutex
	progress     *ImportProgress
}

type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Failed   int      `json:"failed"`
	Messages []string `json:"messages"`
}

type ImportProgress struct {
	Total     int    `json:"total"`
	Processed int    `json:"processed"`
	Imported  int    `json:"imported"`
	Skipped   int    `json:"skipped"`
	Failed    int    `json:"failed"`
	Status    string `json:"status"` // idle/scanning/completed
}

var supportedExtensions = []string{".mp4", ".avi", ".mkv", ".mov", ".flv", ".wmv"}

func NewImportService(cfg *config.Config, videoService *VideoService) *ImportService {
	// Create failed directory
	failedDir := filepath.Join(cfg.Storage.ImportDir, "failed")
	os.MkdirAll(failedDir, 0755)

	return &ImportService{
		config:       cfg,
		videoService: videoService,
		scanning:     false,
		progress: &ImportProgress{
			Status: "idle",
		},
	}
}

// ScanAndImport scans import directory and imports new videos
func (s *ImportService) ScanAndImport() (*ImportResult, error) {
	// Check if already scanning
	s.scanMutex.Lock()
	if s.scanning {
		s.scanMutex.Unlock()
		return nil, fmt.Errorf("扫描已在进行中，请等待当前扫描完成")
	}
	s.scanning = true
	s.scanMutex.Unlock()

	defer func() {
		s.scanMutex.Lock()
		s.scanning = false
		s.progress.Status = "completed"
		s.scanMutex.Unlock()
	}()

	// Step 1: Handle interrupted imports from last scan
	s.recoverInterruptedImports()

	result := &ImportResult{
		Messages: []string{},
	}

	// Step 2: Read import directory
	entries, err := os.ReadDir(s.config.Storage.ImportDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read import dir: %w", err)
	}

	// Filter video files
	var videoFiles []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if isSupportedVideoFile(ext) {
			videoFiles = append(videoFiles, entry)
		}
	}

	// Initialize progress
	s.scanMutex.Lock()
	s.progress = &ImportProgress{
		Total:     len(videoFiles),
		Processed: 0,
		Imported:  0,
		Skipped:   0,
		Failed:    0,
		Status:    "scanning",
	}
	s.scanMutex.Unlock()

	// Step 3: Process each video file
	for _, entry := range videoFiles {
		filename := entry.Name()

		// Check if already imported (by filename)
		var existing models.Video
		if err := database.DB.Where("original_filename = ?", filename).First(&existing).Error; err == nil {
			result.Messages = append(result.Messages, fmt.Sprintf("Skipped already imported: %s", filename))
			result.Skipped++
			s.updateProgress(0, 1, 0)
			continue
		}

		// Import the video
		importPath := filepath.Join(s.config.Storage.ImportDir, filename)
		if err := s.importVideo(importPath, filename, result); err != nil {
			result.Messages = append(result.Messages, fmt.Sprintf("Failed to import %s: %v", filename, err))
			result.Failed++
			s.updateProgress(0, 0, 1)
		} else {
			result.Messages = append(result.Messages, fmt.Sprintf("Successfully imported: %s", filename))
			result.Imported++
			s.updateProgress(1, 0, 0)
		}
	}

	return result, nil
}

// updateProgress updates the import progress
func (s *ImportService) updateProgress(imported, skipped, failed int) {
	s.scanMutex.Lock()
	defer s.scanMutex.Unlock()

	s.progress.Imported += imported
	s.progress.Skipped += skipped
	s.progress.Failed += failed
	s.progress.Processed++
}

// GetProgress returns current import progress
func (s *ImportService) GetProgress() *ImportProgress {
	s.scanMutex.Lock()
	defer s.scanMutex.Unlock()

	// Return a copy to avoid race conditions
	return &ImportProgress{
		Total:     s.progress.Total,
		Processed: s.progress.Processed,
		Imported:  s.progress.Imported,
		Skipped:   s.progress.Skipped,
		Failed:    s.progress.Failed,
		Status:    s.progress.Status,
	}
}

// recoverInterruptedImports handles files that were being processed when last scan was interrupted
func (s *ImportService) recoverInterruptedImports() {
	// Find all logs with "processing" status
	var processingLogs []models.ImportLog
	database.DB.Where("status = ?", "processing").Find(&processingLogs)

	for _, log := range processingLogs {
		// Check if file still exists in import dir
		importPath := filepath.Join(s.config.Storage.ImportDir, log.Filename)
		if _, err := os.Stat(importPath); err == nil {
			// File exists, mark as failed (was interrupted)
			log.Status = "failed"
			log.ErrorMessage = "Import was interrupted"
			log.Message = "Recovered from interrupted import"
			database.DB.Save(&log)

			// Move to failed directory
			failedPath := filepath.Join(s.config.Storage.ImportDir, "failed", log.Filename)
			os.Rename(importPath, failedPath)
		} else {
			// File doesn't exist, might have been partially processed
			// Check if video record exists
			if log.VideoID != nil {
				log.Status = "success"
				log.Message = "Recovered: video was successfully imported"
			} else {
				log.Status = "failed"
				log.ErrorMessage = "File disappeared during import"
				log.Message = "Recovered: file is missing"
			}
			database.DB.Save(&log)
		}
	}
}

func (s *ImportService) importVideo(importPath, filename string, result *ImportResult) error {
	// Get file size
	fileInfo, err := os.Stat(importPath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Create import log with "processing" status
	importLog := &models.ImportLog{
		Filename:   filename,
		Status:     "processing",
		Message:    "Starting import",
		FileSize:   fileInfo.Size(),
		ImportTime: time.Now(),
	}
	database.DB.Create(importLog)

	// Destination path in originals directory
	destPath := filepath.Join(s.config.Storage.OriginalsDir, filename)

	// Move file to originals directory (not copy)
	if err := os.Rename(importPath, destPath); err != nil {
		// If rename fails (different filesystem), copy then delete
		if copyErr := copyFile(importPath, destPath); copyErr != nil {
			s.logImportFailure(importLog, importPath, fmt.Sprintf("failed to move file: %v", copyErr))
			return fmt.Errorf("failed to move file: %w", copyErr)
		}
		os.Remove(importPath) // Remove original after successful copy
	}

	// Generate title from filename (remove extension)
	title := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Create video record
	video, err := s.videoService.CreateVideo(
		destPath,
		filename,
		title,
		"", // empty description
		"import",
		false, // not password protected
		"",
	)

	if err != nil {
		// Move file to failed directory on error
		os.Rename(destPath, importPath) // Move back to import dir first
		s.logImportFailure(importLog, importPath, fmt.Sprintf("failed to create video record: %v", err))
		return err
	}

	// Update log with success status
	importLog.Status = "success"
	importLog.Message = "Successfully imported"
	importLog.VideoID = &video.ID
	database.DB.Save(importLog)

	return nil
}

// logImportFailure logs a failed import and moves file to failed directory
func (s *ImportService) logImportFailure(importLog *models.ImportLog, importPath, errorMsg string) {
	importLog.Status = "failed"
	importLog.ErrorMessage = errorMsg
	importLog.Message = "Import failed"
	database.DB.Save(importLog)

	// Move to failed directory
	failedPath := filepath.Join(s.config.Storage.ImportDir, "failed", filepath.Base(importPath))
	os.Rename(importPath, failedPath)
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
