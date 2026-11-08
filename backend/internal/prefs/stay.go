package prefs

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"
)

// defaultStayDays is how long "stay signed in" lasts for someone who never
// picked a number on the security page.
const defaultStayDays = 30

// StaySignedIn returns how long a session of the user that stays signed in
// lasts after the last visit. It follows the user's setting stayDays, which
// is kept with the user and not per device.
func StaySignedIn(ctx context.Context, db *sql.DB, userID uint64) (time.Duration, error) {
	s, err := loadDoc(ctx, db, `SELECT data FROM user_prefs WHERE user_id = ?`, strconv.FormatUint(userID, 10))
	if err != nil {
		return 0, err
	}
	days := defaultStayDays
	var n float64
	if raw, ok := s["stayDays"]; ok && json.Unmarshal(raw, &n) == nil && n >= 1 {
		days = int(min(n, 365))
	}
	return time.Duration(days) * 24 * time.Hour, nil
}
