package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trainingops/internal/model"
	"trainingops/internal/repository/memory"
)

func TestUploadWrongChecksumReturnsError(t *testing.T) {
	repo := memory.NewContentRepository()
	svc := NewContentService(repo, []byte("secret"), t.TempDir())
	session := model.UploadSession{
		ID:             "s1",
		TenantID:       "tenant-1",
		DocumentID:     "doc-1",
		FileName:       "test.pdf",
		ExpectedChunks: 1,
	}
	if err := svc.StartUpload(context.Background(), session); err != nil {
		t.Fatalf("start upload failed: %v", err)
	}
	_, err := svc.AppendChunk(context.Background(), "tenant-1", "s1", 0, []byte("hello"), "deadbeef")
	if err != ErrUploadChunkChecksum {
		t.Fatalf("expected ErrUploadChunkChecksum, got %v", err)
	}
}

func TestFinalizeIncompleteReturnsError(t *testing.T) {
	repo := memory.NewContentRepository()
	svc := NewContentService(repo, []byte("secret"), t.TempDir())
	session := model.UploadSession{
		ID:             "s2",
		TenantID:       "tenant-1",
		DocumentID:     "doc-2",
		FileName:       "test.pdf",
		ExpectedChunks: 2,
	}
	if err := svc.StartUpload(context.Background(), session); err != nil {
		t.Fatalf("start upload failed: %v", err)
	}
	chunk := []byte("hello")
	sum := sha256.Sum256(chunk)
	checksum := fmt.Sprintf("%x", sum[:])
	_, err := svc.AppendChunk(context.Background(), "tenant-1", "s2", 0, chunk, checksum)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	_, err = svc.FinalizeUpload(context.Background(), "tenant-1", "s2", time.Now().UTC())
	if err != ErrUploadIncomplete {
		t.Fatalf("expected ErrUploadIncomplete, got %v", err)
	}
}

func TestSuccessfulUploadWritesFile(t *testing.T) {
	repo := memory.NewContentRepository()
	storagePath := t.TempDir()
	svc := NewContentService(repo, []byte("secret"), storagePath)
	session := model.UploadSession{
		ID:             "s3",
		TenantID:       "tenant-1",
		DocumentID:     "doc-3",
		FileName:       "test.pdf",
		ExpectedChunks: 1,
	}
	if err := svc.StartUpload(context.Background(), session); err != nil {
		t.Fatalf("start upload failed: %v", err)
	}
	chunk := []byte("hello world")
	sum := sha256.Sum256(chunk)
	checksum := fmt.Sprintf("%x", sum[:])
	_, err := svc.AppendChunk(context.Background(), "tenant-1", "s3", 0, chunk, checksum)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	version, err := svc.FinalizeUpload(context.Background(), "tenant-1", "s3", time.Now().UTC())
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}
	if version.Checksum != checksum {
		t.Fatalf("expected checksum %s, got %s", checksum, version.Checksum)
	}
	path := filepath.Join(storagePath, "doc-3_test.pdf")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file on disk: %v", err)
	}
}

func TestDocumentVersionsIncreaseMonotonically(t *testing.T) {
	repo := memory.NewContentRepository()
	storagePath := t.TempDir()
	svc := NewContentService(repo, []byte("secret"), storagePath)

	runUpload := func(sessionID string, payload []byte) *model.DocumentVersion {
		session := model.UploadSession{
			ID:             sessionID,
			TenantID:       "tenant-1",
			DocumentID:     "doc-versioned",
			FileName:       "module.pdf",
			ExpectedChunks: 1,
		}
		if err := svc.StartUpload(context.Background(), session); err != nil {
			t.Fatalf("start upload failed: %v", err)
		}
		sum := sha256.Sum256(payload)
		checksum := fmt.Sprintf("%x", sum[:])
		if _, err := svc.AppendChunk(context.Background(), "tenant-1", sessionID, 0, payload, checksum); err != nil {
			t.Fatalf("append failed: %v", err)
		}
		version, err := svc.FinalizeUpload(context.Background(), "tenant-1", sessionID, time.Now().UTC())
		if err != nil {
			t.Fatalf("finalize failed: %v", err)
		}
		return version
	}

	v1 := runUpload("ver-1", []byte("first version"))
	v2 := runUpload("ver-2", []byte("second version"))

	if v1.Version != 1 {
		t.Fatalf("expected first version to be 1, got %d", v1.Version)
	}
	if v2.Version != 2 {
		t.Fatalf("expected second version to be 2, got %d", v2.Version)
	}
}
