package web

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/files"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

func TestCheckPermissionsMissingParamsStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts utils.FileOptions
	}{
		{name: "missing path", opts: utils.FileOptions{Source: "default"}},
		{name: "missing source", opts: utils.FileOptions{Path: "/"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := files.CheckPermissions(tt.opts, &users.User{})
			if err == nil {
				t.Fatal("expected an error")
			}
			if status := ErrToStatus(err); status != http.StatusBadRequest {
				t.Fatalf("status=%d err=%v, want %d", status, err, http.StatusBadRequest)
			}
		})
	}
}
