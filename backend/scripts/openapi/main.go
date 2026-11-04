// Writes the merged /api/v1 contract to api/openapi.yaml.
// Run from the backend module root: go run ./scripts/openapi
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/api"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

func main() {
	spec, err := api.NewRouter(v1.Deps{}, nil).OpenAPI()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out := filepath.Join("api", "openapi.yaml")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, spec, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote", out)
}
