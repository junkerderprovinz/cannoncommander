package web

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/files"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/fileutils"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	commonerrors "github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/quota"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
)

func buildUploadQuotaContext(principal *users.User, sourceName, sourcePath, fullIndexPath, realPath, shareHash string, shareLimit int64, sessionID string, totalSize int64, hasTotalSize bool) quota.UploadContext {
	ctx := quota.UploadContext{
		Principal:       principal,
		SourceName:      sourceName,
		SourcePath:      sourcePath,
		DestIndexPath:   fullIndexPath,
		DestRealPath:    realPath,
		ShareHash:       shareHash,
		ShareLimitBytes: shareLimit,
		SessionID:       sessionID,
		TotalSize:       totalSize,
		HasKnownSize:    hasTotalSize,
	}
	if realPath != "" {
		stat, err := os.Stat(realPath)
		if err == nil && !stat.IsDir() {
			ctx.PriorFileSize = stat.Size()
			ctx.HasPriorFileSize = true
		} else if os.IsNotExist(err) {
			ctx.PriorFileSize = 0
			ctx.HasPriorFileSize = true
		}
	}
	return ctx
}

var (
	errPutAccessDenied    = fmt.Errorf("access denied")
	errPutDestDirMissing  = fmt.Errorf("destination directory does not exist")
	errPutPathIsDirectory = fmt.Errorf("path is a directory")
)

// sanitizePutPathError maps path resolution failures to generic client errors (no filesystem paths).
func sanitizePutPathError(err error) (int, error) {
	if err == nil {
		return http.StatusOK, nil
	}
	logger.Debugf("put path resolution failed: %v", err)
	if errors.Is(err, commonerrors.ErrPathEscapesScope) {
		return http.StatusForbidden, errPutAccessDenied
	}
	if strings.Contains(err.Error(), "path is a directory") {
		return http.StatusMethodNotAllowed, errPutPathIsDirectory
	}
	if os.IsNotExist(err) || errors.Is(err, commonerrors.ErrNotExist) {
		return http.StatusNotFound, errPutDestDirMissing
	}
	if strings.Contains(err.Error(), "parent path is not a directory") ||
		strings.Contains(err.Error(), "invalid file path") ||
		isMissingPutTargetError(err) {
		return http.StatusNotFound, errPutDestDirMissing
	}
	return http.StatusNotFound, errPutDestDirMissing
}

// finalizeQuotaPut validates staged bytes, moves the temp file into place, refreshes the index, and commits quota.
// The bool is true when the quota reservation should still be released by the caller.
func finalizeQuotaPut(quotaCtx quota.UploadContext, tempPath, realPath, sourceName string, written, totalSize int64, hasTotalSize bool, contentLength int64) (bool, error) {
	if err := validateReceivedBytes(written, totalSize, hasTotalSize, contentLength); err != nil {
		_ = os.Remove(tempPath)
		return true, err
	}
	if info, err := os.Stat(realPath); err == nil {
		// Do not transfer setuid/setgid bits to new contents without preserving ownership.
		if err = os.Chmod(tempPath, info.Mode().Perm()); err != nil {
			_ = os.Remove(tempPath)
			return true, err
		}
	} else if !os.IsNotExist(err) {
		_ = os.Remove(tempPath)
		return true, err
	}
	if err := fileutils.MoveFile(tempPath, realPath); err != nil {
		_ = os.Remove(tempPath)
		return true, err
	}
	if shouldRefreshIndexAfterPut(sourceName) {
		go files.RefreshIndex(sourceName, filepath.Dir(realPath), true, false) //nolint:errcheck
	}
	if err := commitUploadQuotaAfterMove(quotaCtx); err != nil {
		return false, err
	}
	return false, nil
}

func shouldRefreshIndexAfterPut(sourceName string) bool {
	idx := indexing.GetIndex(sourceName)
	if idx == nil || indexing.GetIndexDB() == nil {
		return false
	}
	return !idx.Config.ResolvedRules.IndexingDisabled
}

func isMissingPutTargetError(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "could not stat resolved path") ||
		strings.Contains(msg, "could not resolve symlinks")
}

func checkUploadQuota(ctx quota.UploadContext) error {
	if err := quota.CheckUploadLength(ctx); err != nil {
		return err
	}
	return quota.ReserveUpload(ctx)
}

func commitUploadQuotaAfterMove(ctx quota.UploadContext) error {
	if err := quota.CommitUpload(ctx); err != nil {
		logger.Warningf("quota commit after upload failed, retrying: %v", err)
		delta := quota.UploadCommitDelta(ctx)
		if retryErr := state.ForceCommitSessionQuota(ctx.SessionID, delta); retryErr != nil {
			return retryErr
		}
	}
	return nil
}

func newQuotaSessionID() string {
	return uuid.New().String()
}

func releaseUploadQuota(sessionID string) {
	quota.ReleaseUpload(sessionID)
}
