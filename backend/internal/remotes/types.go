package remotes

import (
	"cmp"
	"slices"
	"strings"
)

// provider is one backend in rclone's config/providers answer.
type provider struct {
	Name        string   `json:"Name"`
	Description string   `json:"Description"`
	Options     []option `json:"Options"`
	Hide        bool     `json:"Hide"`
}

type option struct {
	Name       string    `json:"Name"`
	Help       string    `json:"Help"`
	Provider   string    `json:"Provider"`
	DefaultStr string    `json:"DefaultStr"`
	Type       string    `json:"Type"`
	Required   bool      `json:"Required"`
	IsPassword bool      `json:"IsPassword"`
	Advanced   bool      `json:"Advanced"`
	Exclusive  bool      `json:"Exclusive"`
	Hide       int       `json:"Hide"`
	Examples   []example `json:"Examples"`
}

type example struct {
	Value    string `json:"Value"`
	Help     string `json:"Help"`
	Provider string `json:"Provider,omitempty"`
}

// hiddenInConfig is rclone's flag for options its own config wizard skips.
const hiddenInConfig = 1

func (p *provider) option(name string) (option, bool) {
	for _, o := range p.Options {
		if o.Name == name {
			return o, true
		}
	}
	return option{}, false
}

// secret reports whether a parameter must not travel back to the browser.
// Passwords are obscured by rclone, but obscured is not encrypted.
func (p *provider) secret(name string) bool {
	if o, ok := p.option(name); ok && o.IsPassword {
		return true
	}
	return secretName(name)
}

func secretName(name string) bool {
	return name == "token" || name == "pass" || name == "password" ||
		strings.Contains(name, "secret") || strings.HasSuffix(name, "_token") || strings.HasSuffix(name, "_pass")
}

// oauth reports whether the backend logs in through a browser, which
// rclone authorize does on a computer that has one.
func (p *provider) oauth() bool {
	_, token := p.option("token")
	_, client := p.option("client_id")
	return token && client
}

// featured are the types the add dialog shows first, in its order.
var featured = []string{"sftp", "smb", "webdav", "s3", "drive", "onedrive", "dropbox"}

// Type is a kind of remote as the add dialog offers it.
type Type struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Featured    bool   `json:"featured"`
	OAuth       bool   `json:"oauth"`
	// Authorize is the command that prints the token on a computer with a
	// browser.
	Authorize string `json:"authorize,omitempty"`
	// PathHint names what the path of a remote of this type is: the share
	// on an SMB server, the bucket on S3, a folder elsewhere.
	PathHint string   `json:"pathHint"`
	Options  []Option `json:"options"`
}

// Option is one setting of a type.
type Option struct {
	Name      string    `json:"name"`
	Help      string    `json:"help"`
	Type      string    `json:"type"`
	Default   string    `json:"default,omitempty"`
	Provider  string    `json:"provider,omitempty"`
	Required  bool      `json:"required"`
	Secret    bool      `json:"secret"`
	Advanced  bool      `json:"advanced"`
	Exclusive bool      `json:"exclusive"`
	Examples  []Example `json:"examples,omitempty"`
}

// Example is a suggested value of an option.
type Example struct {
	Value    string `json:"value"`
	Help     string `json:"help"`
	Provider string `json:"provider,omitempty"`
}

func pathHint(id string) string {
	switch id {
	case "smb":
		return "share"
	case "s3":
		return "bucket"
	}
	return "folder"
}

// types turns the providers into the list the add dialog shows, the
// featured ones first. Advanced options are left out unless asked for.
func types(provs []provider, advanced bool) []Type {
	out := []Type{}
	for _, p := range provs {
		if p.Hide {
			continue
		}
		t := Type{
			ID:          p.Name,
			Description: p.Description,
			Featured:    slices.Contains(featured, p.Name),
			OAuth:       p.oauth(),
			PathHint:    pathHint(p.Name),
			Options:     []Option{},
		}
		if t.OAuth {
			t.Authorize = `rclone authorize "` + p.Name + `"`
		}
		for _, o := range p.Options {
			if o.Hide&hiddenInConfig != 0 || (o.Advanced && !advanced) {
				continue
			}
			opt := Option{
				Name: o.Name, Help: o.Help, Type: o.Type, Default: o.DefaultStr, Provider: o.Provider,
				Required: o.Required, Secret: p.secret(o.Name), Advanced: o.Advanced, Exclusive: o.Exclusive,
			}
			for _, e := range o.Examples {
				opt.Examples = append(opt.Examples, Example(e))
			}
			t.Options = append(t.Options, opt)
		}
		out = append(out, t)
	}
	rank := func(id string) int {
		if i := slices.Index(featured, id); i >= 0 {
			return i
		}
		return len(featured)
	}
	slices.SortStableFunc(out, func(a, b Type) int {
		return cmp.Or(cmp.Compare(rank(a.ID), rank(b.ID)), strings.Compare(strings.ToLower(a.Description), strings.ToLower(b.Description)))
	})
	return out
}
