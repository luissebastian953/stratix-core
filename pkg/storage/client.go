package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Client defines the interface for object storage operations.
// Backed by Cloudflare R2 or MinIO (both S3-compatible APIs).
// The interface is defined here so tests can inject a fake implementation
// without making real HTTP calls.
type Client interface {
	// Upload stores an object at the given key with the given content type.
	Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error

	// Delete removes an object by key. Returns nil if the key does not exist.
	Delete(ctx context.Context, key string) error

	// PresignURL generates a time-limited URL for direct client downloads.
	// The mobile app fetches attachments directly from Cloudflare's CDN using
	// this URL — no attachment bytes flow through the Go server.
	PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}

// ── Key generation ────────────────────────────────────────────────────────────
// Storage keys are structured paths that organise objects by user and task.
// This makes it easy to list or delete all objects for a user/task if needed.
//
// Format: attachments/{userID}/{taskID}/{uuid}.{ext}

// NewAttachmentKey generates a unique storage key for a task attachment.
// userID and taskID scope the object so access patterns are predictable.
func NewAttachmentKey(userID, taskID uuid.UUID, filename string) string {
	ext := fileExtension(filename)
	objectID := uuid.New().String()
	return fmt.Sprintf("attachments/%s/%s/%s%s", userID, taskID, objectID, ext)
}

// UserPrefix returns the storage prefix for all objects belonging to a user.
// Useful for listing or bulk-deleting a user's attachments on account deletion.
func UserPrefix(userID uuid.UUID) string {
	return fmt.Sprintf("attachments/%s/", userID)
}

// TaskPrefix returns the storage prefix for all attachments on a task.
func TaskPrefix(userID, taskID uuid.UUID) string {
	return fmt.Sprintf("attachments/%s/%s/", userID, taskID)
}

func fileExtension(filename string) string {
	ext := path.Ext(filename)
	if ext == "" {
		return ""
	}
	// Sanitise — only allow alphanumeric extensions
	clean := strings.ToLower(strings.TrimPrefix(ext, "."))
	for _, c := range clean {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return ""
		}
	}
	return "." + clean
}

// ── Validation ────────────────────────────────────────────────────────────────

const (
	// MaxAttachmentSize is the maximum allowed file size for uploads.
	// 25MB — generous for images and documents, restrictive enough to
	// prevent storage abuse.
	MaxAttachmentSize = 25 * 1024 * 1024 // 25MB in bytes

	// PresignURLExpiry is the default lifetime of a presigned download URL.
	// 1 hour is long enough for a user to tap and view, short enough to
	// limit exposure if the URL leaks.
	PresignURLExpiry = time.Hour
)

// AllowedContentTypes is the set of MIME types accepted for attachments.
// Reject everything else to prevent storing executable files.
var AllowedContentTypes = map[string]bool{
	// Images
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	"image/heic": true,

	// Documents
	"application/pdf": true,
	"text/plain":      true,
	"text/markdown":   true,

	// Office
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true, // .docx
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true, // .xlsx
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true, // .pptx
}

// ValidateUpload checks size and content type before uploading.
func ValidateUpload(size int64, contentType string) error {
	if size <= 0 {
		return fmt.Errorf("file is empty")
	}
	if size > MaxAttachmentSize {
		return fmt.Errorf("file size %d bytes exceeds maximum of %d bytes", size, MaxAttachmentSize)
	}
	// Strip parameters e.g. "image/jpeg; charset=utf-8" → "image/jpeg"
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if !AllowedContentTypes[ct] {
		return fmt.Errorf("content type %q is not allowed", ct)
	}
	return nil
}

// ── Fake client for tests ─────────────────────────────────────────────────────

// FakeClient is an in-memory Client implementation for use in tests.
// It stores uploaded content in a map and returns predictable presigned URLs.
type FakeClient struct {
	Objects map[string][]byte // key → content
}

func NewFakeClient() *FakeClient {
	return &FakeClient{Objects: make(map[string][]byte)}
}

func (f *FakeClient) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.Objects[key] = data
	return nil
}

func (f *FakeClient) Delete(_ context.Context, key string) error {
	delete(f.Objects, key)
	return nil
}

func (f *FakeClient) PresignURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://fake-storage.example.com/" + key, nil
}

func (f *FakeClient) Exists(key string) bool {
	_, ok := f.Objects[key]
	return ok
}
