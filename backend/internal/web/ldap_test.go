package web

import (
	"testing"
)

func TestLdapGroupMatches(t *testing.T) {
	tests := []struct {
		name       string
		member     string
		configured string
		want       bool
	}{
		{
			name:       "exact DN match",
			member:     "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			configured: "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			want:       true,
		},
		{
			name:       "CN match",
			member:     "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			configured: "authentik Admins",
			want:       true,
		},
		{
			name:       "CN match case-insensitive",
			member:     "cn=it department,ou=groups,dc=example,dc=com",
			configured: "IT Department",
			want:       true,
		},
		{
			name:       "full DN match case-insensitive",
			member:     "CN=Employees,OU=Groups,DC=Example,DC=COM",
			configured: "cn=employees,ou=groups,dc=example,dc=com",
			want:       true,
		},
		{
			name:       "config full DN vs member CN-only",
			member:     "Employees",
			configured: "cn=Employees,ou=groups,dc=example,dc=com",
			want:       true,
		},
		{
			name:       "CN no match",
			member:     "cn=Other Group,ou=groups,dc=test",
			configured: "authentik Admins",
			want:       false,
		},
		{
			name:       "exact no match",
			member:     "cn=admins,ou=groups,dc=test",
			configured: "cn=authentik Admins,ou=groups,dc=ldap,dc=goauthentik,dc=io",
			want:       false,
		},
		{
			name:       "same CN different parents rejected",
			member:     "cn=Admins,ou=Other,dc=example,dc=com",
			configured: "cn=Admins,ou=Privileged,dc=example,dc=com",
			want:       false,
		},
		{
			name:       "whitespace trimmed",
			member:     "  cn=admins,ou=groups,dc=test  ",
			configured: "cn=admins,ou=groups,dc=test",
			want:       true,
		},
		{
			name:       "invalid DN returns false",
			member:     "not-a-valid-dn",
			configured: "admins",
			want:       false,
		},
		{
			name:       "empty configured no match",
			member:     "cn=admins,ou=groups,dc=test",
			configured: "",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ldapGroupMatches(tt.member, tt.configured)
			if got != tt.want {
				t.Errorf("ldapGroupMatches(%q, %q) = %v, want %v", tt.member, tt.configured, got, tt.want)
			}
		})
	}
}
