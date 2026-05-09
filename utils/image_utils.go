package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxFileSize defines the maximum allowed file size for image uploads (10MB).
	MaxFileSize = 10 << 20 // 10MB
	// UploadDir defines the directory where uploaded images are stored.
	UploadDir = "uploads"
)

// AllowedImageTypes defines the permitted image MIME types for upload.
// Only common web-safe image formats are allowed for security.
var AllowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

// IsValidImageType checks if the provided content type is an allowed image type.
// Returns true if the content type is in the AllowedImageTypes map.
func IsValidImageType(contentType string) bool {
	return AllowedImageTypes[strings.ToLower(contentType)]
}

// GenerateFileName creates a unique filename for uploaded images.
// Uses cryptographic randomness to ensure uniqueness and security.
// Preserves the original file extension or defaults to .jpg.
func GenerateFileName(originalName string) (string, error) {
	// Generate random bytes for unique filename
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Get file extension
	ext := filepath.Ext(originalName)
	if ext == "" {
		ext = ".jpg" // default extension
	}

	// Create unique filename
	filename := hex.EncodeToString(bytes) + ext
	return filename, nil
}

// SaveUploadedImage saves an uploaded image file to the uploads directory.
// Validates file size and type before saving.
// Returns the generated filename on success, or an error on failure.
func SaveUploadedImage(file multipart.File, header *multipart.FileHeader) (string, error) {
	// Validate file size
	if header.Size > MaxFileSize {
		return "", fmt.Errorf("file size exceeds maximum allowed size of %d bytes", MaxFileSize)
	}

	// Validate file type
	if !IsValidImageType(header.Header.Get("Content-Type")) {
		return "", fmt.Errorf("invalid file type: %s", header.Header.Get("Content-Type"))
	}

	// Generate unique filename
	filename, err := GenerateFileName(header.Filename)
	if err != nil {
		return "", fmt.Errorf("failed to generate filename: %v", err)
	}

	// Create the full file path
	filePath := filepath.Join(UploadDir, filename)

	// Create the destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %v", err)
	}
	defer dst.Close()

	// Reset file pointer to beginning
	file.Seek(0, 0)

	// Copy the uploaded file to the destination
	_, err = io.Copy(dst, file)
	if err != nil {
		// Clean up the file if copy failed
		os.Remove(filePath)
		return "", fmt.Errorf("failed to save file: %v", err)
	}

	return filename, nil
}

// DeleteImage removes an image file from the uploads directory.
// Returns nil if the filename is empty (nothing to delete).
// Returns an error if the file cannot be deleted.
func DeleteImage(filename string) error {
	if filename == "" {
		return nil // Nothing to delete
	}

	filePath := filepath.Join(UploadDir, filename)
	return os.Remove(filePath)
}

// ImageExists checks if an image file exists in the uploads directory.
// Returns false if the filename is empty or the file doesn't exist.
// Returns true if the file exists and is accessible.
func ImageExists(filename string) bool {
	if filename == "" {
		return false
	}

	filePath := filepath.Join(UploadDir, filename)
	_, err := os.Stat(filePath)
	return err == nil
}
