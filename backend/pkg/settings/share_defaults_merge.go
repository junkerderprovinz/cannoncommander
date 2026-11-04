package settings

// ErrEnforcedShareValueMismatch is returned when a share field differs from the enforced default value.
type ErrEnforcedShareValueMismatch struct {
	Path string
}
