package web

import (
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"time"

	_ "net/http/pprof"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/fileutils"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/events"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// Deps holds injected dependencies for HTTP handlers and middleware.
type Deps struct {
	Store *state.Store
	Auth  *auth.Service
}

var (
	runtimeDeps Deps
	assetFs     fs.FS
)

// Embed the files in the frontend/dist directory
//
//go:embed embed/*
var assets embed.FS

// GetEmbeddedAssets returns the embedded assets filesystem
func GetEmbeddedAssets() embed.FS {
	return assets
}

// StartHttp starts the HTTP server and blocks until ctx is cancelled.
func StartHttp(ctx context.Context, deps Deps, shutdownComplete chan struct{}) {
	if settings.Env.IsDevMode {
		go func() {
			if err := http.ListenAndServe("localhost:6060", nil); err != nil {
				logger.Fatalf("pprof server error: %v", err)
			}
		}()
	}

	assetFs = fileutils.GetAssetFS()
	if assetFs == nil {
		logger.Fatal("Asset filesystem not initialized. Call fileutils.InitAssetFS first.")
	}
	runtimeDeps = deps

	router := http.NewServeMux()
	configureHTTPRouter(router)
	if err := mountAPIv1(router); err != nil {
		logger.Fatalf("Could not set up /api/v1: %v", err)
	}

	srv := &http.Server{
		Addr:        settings.HTTPListenAddr(settings.Config.Http.ListenAddress, settings.Config.Http.Port),
		Handler:     muxWithMiddleware(router),
		ConnContext: settings.AttachConnContext,
	}
	listenAddress := settings.Config.Http.ListenAddress
	go func() {
		var listener net.Listener
		var err error

		if settings.Config.Http.Socket != "" {
			if err = os.Remove(settings.Config.Http.Socket); err != nil && !os.IsNotExist(err) {
				logger.Fatalf("Could not remove existing socket: %v", err)
			}
			listener, err = settings.ListenUnixFilesystem(settings.Config.Http.Socket)
			if err != nil {
				logger.Fatalf("Could not listen on unix socket: %v", err)
			}
			logger.Infof("Running at               : unix://%s%s", settings.Config.Http.Socket, settings.Config.Http.BaseURL)
		} else if settings.Config.Http.TLSCert != "" && settings.Config.Http.TLSKey != "" {
			cer, err := tls.LoadX509KeyPair(settings.Config.Http.TLSCert, settings.Config.Http.TLSKey)
			if err != nil {
				logger.Fatalf("Could not load certificate: %v", err)
			}
			tlsConfig := &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cer},
			}
			scheme := "https"
			port := ""
			if settings.Config.Http.Port != 443 {
				port = fmt.Sprintf(":%d", settings.Config.Http.Port)
			}
			fullURL := fmt.Sprintf("%s://%s%s%s", scheme, listenAddress, port, settings.Config.Http.BaseURL)
			logger.Infof("Running at               : %s", fullURL)

			socketActivationListeners, err := activation.TLSListeners(tlsConfig)
			if err == nil && len(socketActivationListeners) > 0 {
				listener = socketActivationListeners[0]
				logger.Debug("Socket activation detected. Listening address is being controlled by systemd.")
			} else if err != nil {
				logger.Debugf("Socket activation failed: %v", err)
			}
			if listener == nil {
				listener, err = tls.Listen("tcp", srv.Addr, tlsConfig)
				if err != nil {
					logger.Fatalf("Could not start TLS server: %v", err)
				}
			}
		} else {
			scheme := "http"
			port := ""
			if settings.Config.Http.Port != 80 {
				port = fmt.Sprintf(":%d", settings.Config.Http.Port)
			}
			fullURL := fmt.Sprintf("%s://%s%s%s", scheme, listenAddress, port, settings.Config.Http.BaseURL)
			logger.Infof("Running at               : %s", fullURL)

			socketActivationListeners, err := activation.Listeners()
			if err == nil && len(socketActivationListeners) > 0 {
				listener = socketActivationListeners[0]
				logger.Debug("Socket activation detected. Listening address is being controlled by systemd.")
			} else if err != nil {
				logger.Debugf("Socket activation failed: %v", err)
			}
			if listener == nil {
				addr := srv.Addr
				if addr == "" {
					addr = ":http"
				}
				listener, err = net.Listen("tcp", addr)
				if err != nil {
					logger.Fatalf("Server error: %v", err)
				}
			}
		}

		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down HTTP server...")
	events.Shutdown()
	eventHub.Close()

	if err := state.Close(); err != nil {
		logger.Errorf("Failed to close state management: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("HTTP server forced to shut down: %v", err)
	} else {
		logger.Info("HTTP server shut down gracefully.")
	}

	if settings.Config.Http.Socket != "" {
		if err := os.Remove(settings.Config.Http.Socket); err != nil && !os.IsNotExist(err) {
			logger.Debugf("Could not remove unix socket: %v", err)
		}
	}

	close(shutdownComplete)
}
