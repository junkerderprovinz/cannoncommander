//go:build !desktop

package terminal

import (
	"net/http"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

// native is empty: a server in a container has no screen to open a terminal
// window on.
type native struct{}

func newNative() native { return native{} }

func (h *handlers) openNative(w http.ResponseWriter, r *http.Request) error {
	return v1.DesktopOnly(w, r)
}
