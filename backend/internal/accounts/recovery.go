package accounts

import (
	"context"
	"crypto/rand"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

// codeAlphabet leaves out the letters and digits that look alike, so codes
// read off paper or a screen come back right.
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

const recoveryCount = 8

// randomCode returns n random characters from codeAlphabet.
func randomCode(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i := range b {
		// 256 is a multiple of 32, so every character is equally likely.
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b)
}

// normalizeCode takes a code as people type it: any case, with spaces or
// dashes. It returns "" when the result has the wrong length or characters.
func normalizeCode(code string, n int) string {
	c := strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(code)))
	if len(c) != n {
		return ""
	}
	for _, ch := range c {
		if !strings.ContainsRune(codeAlphabet, ch) {
			return ""
		}
	}
	return c
}

// recovery keeps the one time codes that stand in for a TOTP code. Only
// bcrypt hashes are stored, because a code has just 40 bits.
type recovery struct {
	db *sql.DB
}

// issue replaces every code of the user with recoveryCount new ones and
// returns them as XXXX-XXXX.
func (c *recovery) issue(ctx context.Context, userID uint64) ([]string, error) {
	codes := make([]string, recoveryCount)
	hashes := make([]string, recoveryCount)
	for i := range codes {
		plain := randomCode(8)
		h, err := utils.HashPwd(plain)
		if err != nil {
			return nil, err
		}
		codes[i], hashes[i] = plain[:4]+"-"+plain[4:], h
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	id := strconv.FormatUint(userID, 10)
	if _, err := tx.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, id); err != nil {
		return nil, err
	}
	for _, h := range hashes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)`, id, h); err != nil {
			return nil, err
		}
	}
	return codes, tx.Commit()
}

func (c *recovery) left(ctx context.Context, userID uint64) (int, error) {
	if c.db == nil {
		return 0, nil
	}
	var n int
	err := c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM recovery_codes WHERE user_id = ? AND used_at = 0`, strconv.FormatUint(userID, 10)).Scan(&n)
	return n, err
}

func (c *recovery) clear(ctx context.Context, userID uint64) error {
	_, err := c.db.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, strconv.FormatUint(userID, 10))
	return err
}

// Redeem uses up one code of the user. Two requests with the same code can
// race, and the update lets only one of them win.
func (c *recovery) Redeem(userID uint64, code string) bool {
	plain := normalizeCode(code, 8)
	if plain == "" || c.db == nil {
		return false
	}
	ctx := context.Background()
	rows, err := c.db.QueryContext(ctx, `SELECT id, code_hash FROM recovery_codes WHERE user_id = ? AND used_at = 0`, strconv.FormatUint(userID, 10))
	if err != nil {
		logger.Errorf("read recovery codes: %v", err)
		return false
	}
	var match int64
	for rows.Next() {
		var id int64
		var hash string
		if err := rows.Scan(&id, &hash); err != nil {
			break
		}
		if match == 0 && utils.CheckPwd(plain, hash) == nil {
			match = id
		}
	}
	rows.Close()
	if match == 0 {
		return false
	}
	res, err := c.db.ExecContext(ctx, `UPDATE recovery_codes SET used_at = ? WHERE id = ? AND used_at = 0`, time.Now().Unix(), match)
	if err != nil {
		logger.Errorf("use recovery code: %v", err)
		return false
	}
	n, err := res.RowsAffected()
	return err == nil && n == 1
}
