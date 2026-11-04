package state

import (
	"fmt"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/access"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/dbindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/sqldb"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

var (
	// Serializes Open/Close so tests and startup cannot interleave lifecycle or sqlDb swaps.
	stateLifecycleMu sync.Mutex

	// Separate mutexes for each resource type for better concurrency
	usersMux sync.RWMutex
	indexMux sync.RWMutex

	// sqlDb is the SQLite persistence layer. Only state package code should call it directly;
	// everyone else goes through exported state.* helpers and the caches below.
	sqlDb *sqldb.SQLStore

	// In-memory caches (authoritative at runtime after Initialize for access and index; users use TTL cache)
	indexInfoByPath map[string]*dbindex.IndexInfo

	// accessDb holds access rules, groups, and token hashes in memory with write-through to sqlDb.
	accessDb *access.Storage
)

// Initialize loads all data from SQL into memory and sets the default store handle.
// Prefer state.Open when you need the *Store for dependency injection.
func Initialize(dbPath string) (bool, error) {
	_, existingDb, err := Open(dbPath)
	return existingDb, err
}

func initialize(dbPath string) (bool, error) {
	if !settings.Env.IsCLIMode {
		logger.Info("Initializing state management system...")
	}

	store, existingDb, err := sqldb.NewSQLStore(dbPath)
	if err != nil {
		return false, fmt.Errorf("failed to initialize SQL database: %w", err)
	}
	sqlDb = store

	if err = InitSourceAccessDefaults(); err != nil {
		return existingDb, fmt.Errorf("failed to initialize source access defaults: %w", err)
	}

	if err = InitAuthSigningKey(); err != nil {
		return existingDb, fmt.Errorf("failed to initialize auth signing key: %w", err)
	}

	var userCount int
	if countErr := sqlDb.DB().QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount); countErr != nil {
		return existingDb, fmt.Errorf("failed to count users: %w", countErr)
	}
	if userCount == 0 {
		if err = QuickSetup(); err != nil {
			return existingDb, fmt.Errorf("failed to run initial setup: %w", err)
		}
	}

	logger.Debugf("Loading index data into memory...")

	users.SetUsernameToID(UserIDForUsername)

	newIndexInfoByPath := make(map[string]*dbindex.IndexInfo)
	allIndexInfo, err := sqlDb.ListAllIndexInfo()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load index info: %w", err)
	}
	for _, info := range allIndexInfo {
		newIndexInfoByPath[info.Path] = info
	}
	logger.Debugf("Loaded %d index info entries", len(allIndexInfo))

	newAccessDb := &access.Storage{
		Groups:        make(access.GroupMap),
		RevokedTokens: make(map[string]int64),
		HashedTokens:  make(map[string]access.HashedTokenInfo),
	}

	allGroups, err := sqlDb.GetAllGroups()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load groups: %w", err)
	}
	newAccessDb.Groups = allGroups
	logger.Debugf("Loaded %d groups", len(allGroups))

	revokedTokens, err := sqlDb.GetAllRevokedTokens()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load revoked tokens: %w", err)
	}
	newAccessDb.RevokedTokens = revokedTokens
	logger.Debugf("Loaded %d revoked tokens", len(revokedTokens))

	hashedTokens, err := sqlDb.GetAllHashedTokens()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load hashed tokens: %w", err)
	}
	newAccessDb.HashedTokens = make(map[string]access.HashedTokenInfo, len(hashedTokens))
	now := time.Now()
	expired := 0
	for hash, record := range hashedTokens {
		if access.TokenExpiredPastGrace(record.ExpiresAt, now) {
			expired++
			if err = sqlDb.DeleteHashedToken(hash); err != nil {
				logger.Errorf("failed to delete expired token hash from sql: %v", err)
			}
			continue
		}
		newAccessDb.HashedTokens[hash] = access.HashedTokenInfo{
			UserID:    record.UserID,
			IsSession: record.IsSession,
			ExpiresAt: record.ExpiresAt,
		}
	}
	logger.Debugf("Loaded %d hashed tokens (%d expired dropped)", len(hashedTokens)-expired, expired)

	newAccessDb.SetSQLStore(sqlDb)

	usersMux.Lock()
	indexMux.Lock()
	indexInfoByPath = newIndexInfoByPath
	accessDb = newAccessDb
	usersMux.Unlock()
	indexMux.Unlock()

	if err = BackfillHashedTokensFromUserRecords(); err != nil {
		return existingDb, fmt.Errorf("failed to backfill hashed tokens: %w", err)
	}

	err = auth.InitializeEncryption()
	if err != nil {
		return existingDb, fmt.Errorf("failed to initialize auth encryption: %w", err)
	}

	logger.Debugf("State management system initialized successfully")

	InitActivityRecorder(settings.Config.Server.DatabaseV2)

	return existingDb, nil
}

// Close closes the underlying SQL database
func Close() error {
	stateLifecycleMu.Lock()
	defer stateLifecycleMu.Unlock()

	usersMux.Lock()
	indexMux.Lock()
	defer usersMux.Unlock()
	defer indexMux.Unlock()

	users.SetUsernameToID(nil)
	clearUserRecordCache()
	StopActivityRecorder()

	var err error
	if sqlDb != nil {
		err = sqlDb.Close()
		sqlDb = nil
	}
	defaultStore = nil
	return err
}
