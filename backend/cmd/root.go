package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "net/http/pprof"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/fileutils"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/app"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/preview"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/version"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/web"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func initializeDatabase(configFile string, cliMode bool) bool {
	if cliMode {
		settings.InitializeCLI(configFile)
	} else {
		settings.Initialize(configFile)
	}

	store, existingDb, err := state.Open(settings.Config.Server.DatabaseV2.Path)
	if err != nil {
		logger.Fatalf("could not initialize state: %v", err)
	}
	if _, err = app.WireServices(store); err != nil {
		logger.Fatalf("could not wire services: %v", err)
	}
	return existingDb
}

func Start() {
	legacyEnv := adoptLegacyEnv()
	keepGoing, dbExists := runCLI()
	if !keepGoing {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalChan
		logger.Info("Received shutdown signal. Shutting down gracefully...")
		cancel()
	}()
	serve(ctx, dbExists, legacyEnv)
	if system.RestartWanted() {
		logger.Info("Starting again.")
		if err := system.Restart(); err != nil {
			logger.Fatalf("Could not start again: %v", err)
		}
	}
}

// serve runs the server until ctx ends and then shuts it down.
func serve(ctx context.Context, dbExists bool, legacyEnv []string) {
	system.ApplyStored(context.Background(), state.MainDB())
	database := fmt.Sprintf("Using existing database  : %v", settings.Config.Server.DatabaseV2.Path)
	if !dbExists {
		database = fmt.Sprintf("Creating new database    : %v", settings.Config.Server.DatabaseV2.Path)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan struct{})             // Signals server has stopped
	shutdownComplete := make(chan struct{}) // Signals shutdown process is complete

	// Dev mode enables development features like template hot-reloading
	_, err := os.Stat("internal/web/dist")
	// In dev mode, always use filesystem assets. Otherwise, check if internal/web/dist exists
	if !settings.Env.IsDevMode {
		settings.Env.EmbeddedFs = os.IsNotExist(err)
	}

	err = utils.SetInvalidPasswordHash()
	if err != nil {
		logger.Fatalf("Failed to set security hash: %v", err)
	}

	sourceList := []string{}
	for path, source := range settings.Config.Server.SourceMap {
		sourceList = append(sourceList, fmt.Sprintf("%v: %v", source.Name, path))
	}
	logger.Infof("Initializing CannonCommander (%v)", version.Version)
	for _, name := range legacyEnv {
		logger.Warningf("%s is a FileBrowser name, read it as %s%s. Please rename it.", name, envPrefix, strings.TrimPrefix(name, legacyEnvPrefix))
	}
	logger.Infof("Using Config file        : %v", configPath)
	logger.Infof("Auth Methods             : %v", settings.Config.Auth.AuthMethods)
	logger.Info(database)
	logger.Infof("Sources                  : %v", sourceList)
	logger.Debugf("Using Embedded FS        : %v", settings.Env.EmbeddedFs)
	walModeStr := "OFF"
	if settings.Config.Server.IndexSqlConfig.WalMode {
		walModeStr = "WAL"
	}
	logger.Infof("SQL Journal Mode         : %v", walModeStr)
	if settings.Config.Server.CacheDirCleanup {
		logger.Debugf("clearing cache dir: %s", settings.Config.Server.CacheDir)
		fileutils.ClearCacheDir(settings.Config.Server.CacheDir)
	}
	serverConfig := settings.Config.Server
	// initialize indexing and schedule indexing ever n minutes (default 5)
	if len(settings.Config.Server.SourceMap) == 0 {
		logger.Fatal("No sources configured, exiting...")
	}

	// Initialize shared index database before starting HTTP service
	isNewDb, err := indexing.InitializeIndexDB()
	if err != nil {
		logger.Fatalf("Failed to initialize index database: %v", err)
	}

	// Index metadata persistence is wired from app.WireServices during initializeDatabase.
	if isNewDb {
		if err := state.ResetAllIndexComplexities(); err != nil {
			logger.Errorf("Failed to reset index complexities: %v", err)
		}
	}

	indexing.LinkedFolder = unraid.Default().LinkedFolder
	for _, source := range settings.Config.Server.SourceMap {
		go indexing.Initialize(source, false, isNewDb)
	}
	validateUserInfo(!dbExists)
	validateOfficeIntegration()
	// Start the rootCMD in a goroutine
	go func() {
		if err := rootCMD(ctx, &serverConfig, shutdownComplete); err != nil {
			logger.Fatalf("Error starting the server: %v", err)
		}
		close(done) // Signal that the server has stopped
	}()
	// Wait for the end of ctx or for the server to stop
	select {
	case <-ctx.Done():
	case <-done:
		logger.Info("Server stopped unexpectedly. Shutting down...")
		cancel()
	}

	// Stop all indexing scanners before closing the database
	indexing.StopAllScanners()

	// cleanup temp databases
	indexDB := indexing.GetIndexDB()
	if indexDB != nil {
		indexDB.Close()
	}
	if settings.Config.Server.CacheDirCleanup {
		logger.Debugf("clearing cache dir: %s", settings.Config.Server.CacheDir)
		fileutils.ClearCacheDir(settings.Config.Server.CacheDir)
	}
	<-shutdownComplete
	system.Shutdown()
	if err := fileutils.ClearDirectoryContents(settings.DownloadCacheDir()); err != nil {
		logger.Warningf("failed to clear download spool on shutdown: %v", err)
	}
	logger.Info("Shutdown complete.")
}

func rootCMD(ctx context.Context, serverConfig *settings.Server, shutdownComplete chan struct{}) error {
	if serverConfig.NumImageProcessors < 1 {
		logger.Fatal("Image resize workers count could not be < 1")
	}
	cacheDir := settings.Config.Server.CacheDir
	numWorkers := settings.Config.Server.NumImageProcessors

	// Initialize asset filesystem before starting services
	if settings.Env.EmbeddedFs {
		embeddedAssets := web.GetEmbeddedAssets()
		subAssets, err := fs.Sub(embeddedAssets, "embed")
		if err != nil {
			logger.Fatalf("Failed to create sub filesystem: %v", err)
		}
		fileutils.InitAssetFS(subAssets, true, false)
	} else {
		fileutils.InitAssetFS(nil, false, settings.Env.IsDevMode)
	}

	// Start preview service
	err := preview.StartPreviewGenerator(numWorkers, cacheDir)
	if err != nil {
		logger.Fatalf("Error starting preview service: %v", err)
	}
	logger.Debugf("MuPDF Enabled            : %v", settings.Env.MuPdfAvailable)

	web.StartHttp(ctx, shutdownComplete)
	return nil
}
