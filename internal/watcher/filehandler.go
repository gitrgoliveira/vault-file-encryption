package watcher

import (
	"os"
	"path/filepath"

	"github.com/gitrgoliveira/vault_file_encryption/internal/logger"
	"github.com/gitrgoliveira/vault_file_encryption/internal/model"
)

// FileHandler manages post-processing file operations
type FileHandler struct {
	logger             logger.Logger
	sourceFileBehavior string
	archiveDir         string
	failedDir          string
	dlqDir             string
}

// FileHandlerConfig holds file handler configuration
type FileHandlerConfig struct {
	SourceFileBehavior string
	ArchiveDir         string
	FailedDir          string
	DLQDir             string
}

// NewFileHandler creates a new file handler
func NewFileHandler(cfg *FileHandlerConfig, log logger.Logger) (*FileHandler, error) {
	// Create directories if they don't exist
	for _, dir := range []string{cfg.ArchiveDir, cfg.FailedDir, cfg.DLQDir} {
		if dir != "" {
			if err := os.MkdirAll(dir, 0750); err != nil { // #nosec G301 - configurable directory path
				return nil, err
			}
		}
	}

	return &FileHandler{
		logger:             log,
		sourceFileBehavior: cfg.SourceFileBehavior,
		archiveDir:         cfg.ArchiveDir,
		failedDir:          cfg.FailedDir,
		dlqDir:             cfg.DLQDir,
	}, nil
}

// UpdateConfig updates the file handler's configuration
func (fh *FileHandler) UpdateConfig(cfg *FileHandlerConfig) {
	fh.sourceFileBehavior = cfg.SourceFileBehavior
	fh.archiveDir = cfg.ArchiveDir
	fh.failedDir = cfg.FailedDir
	fh.dlqDir = cfg.DLQDir

	// Re-create directories if they don't exist
	for _, dir := range []string{fh.archiveDir, fh.failedDir, fh.dlqDir} {
		if dir != "" {
			if err := os.MkdirAll(dir, 0750); err != nil { // #nosec G301
				fh.logger.Error("Failed to create directory on config update", "dir", dir, "error", err)
			}
		}
	}
}

// HandleSourceFile handles the source file after successful processing
func (fh *FileHandler) HandleSourceFile(sourcePath string) {
	switch fh.sourceFileBehavior {
	case "delete":
		if err := os.Remove(sourcePath); err != nil {
			fh.logger.Error("Failed to delete source file", "file", sourcePath, "error", err)
		} else {
			fh.logger.Info("Deleted source file", "file", sourcePath)
		}

	case "archive":
		fileName := filepath.Base(sourcePath)
		archivePath := filepath.Join(fh.archiveDir, fileName)

		if err := os.Rename(sourcePath, archivePath); err != nil {
			fh.logger.Error("Failed to archive source file", "file", sourcePath, "error", err)
		} else {
			fh.logger.Info("Archived source file", "file", sourcePath, "archive", archivePath)
		}

	default:
		fh.logger.Error("Unknown source file behavior", "behavior", fh.sourceFileBehavior)
	}
}

// MoveToFailed moves a file to the failed directory
func (fh *FileHandler) MoveToFailed(sourcePath string) {
	if fh.failedDir == "" {
		return
	}

	fileName := filepath.Base(sourcePath)
	failedPath := filepath.Join(fh.failedDir, fileName)

	if err := os.Rename(sourcePath, failedPath); err != nil {
		fh.logger.Error("Failed to move file to failed directory", "file", sourcePath, "error", err)
	} else {
		fh.logger.Info("Moved file to failed directory", "file", sourcePath, "failed", failedPath)
	}
}

// MoveToDLQ moves an item's file to the dead letter queue
func (fh *FileHandler) MoveToDLQ(item *model.Item) {
	if fh.dlqDir == "" {
		return
	}

	fileName := filepath.Base(item.SourcePath)
	dlqPath := filepath.Join(fh.dlqDir, fileName)

	if err := os.Rename(item.SourcePath, dlqPath); err != nil {
		fh.logger.Error("Failed to move file to DLQ", "file", item.SourcePath, "error", err)
	} else {
		fh.logger.Info("Moved file to DLQ", "file", item.SourcePath, "dlq", dlqPath)
	}
}
