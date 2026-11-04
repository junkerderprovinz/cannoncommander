package sqldb

import (
	"os"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.Main(m))
}
