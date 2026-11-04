package web

import (
	"strings"

	ldap "github.com/go-ldap/ldap/v3"
)

// ldapGroupMatches reports whether an LDAP group membership value matches a
// configured group. Matching is case-insensitive and accepts a full DN or a
// CN-only value. When both sides are DNs, the complete DN must match (case-
// insensitive); CN-only fallback applies only when one side is not a DN, so
// groups that share a CN under different parents cannot authorize each other.
func ldapGroupMatches(member, configured string) bool {
	member = strings.TrimSpace(member)
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return false
	}
	if strings.EqualFold(member, configured) {
		return true
	}
	memberIsDN := ldapIsDN(member)
	configuredIsDN := ldapIsDN(configured)
	memberCN := ldapCN(member)
	configuredCN := ldapCN(configured)

	// Config CN-only vs member full DN.
	if memberIsDN && !configuredIsDN && memberCN != "" && strings.EqualFold(memberCN, configured) {
		return true
	}
	// Config full DN vs member CN-only.
	if configuredIsDN && !memberIsDN && configuredCN != "" && strings.EqualFold(member, configuredCN) {
		return true
	}
	return false
}

// ldapIsDN reports whether s parses as an LDAP distinguished name.
func ldapIsDN(s string) bool {
	_, err := ldap.ParseDN(s)
	return err == nil
}

// ldapCN returns the first CN RDN value from an LDAP DN string, or "" if none.
func ldapCN(s string) string {
	dn, err := ldap.ParseDN(s)
	if err != nil {
		return ""
	}
	for _, rdn := range dn.RDNs {
		for _, attr := range rdn.Attributes {
			if strings.EqualFold(attr.Type, "cn") {
				return attr.Value
			}
		}
	}
	return ""
}
