package state

import (
	"os"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	utils.BcryptCost = bcrypt.MinCost
	os.Exit(testutil.Main(m))
}
