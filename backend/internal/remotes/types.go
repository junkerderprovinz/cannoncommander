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
	Sensitive  bool      `json:"Sensitive"`
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
// Passwords are obscured by rclone, but obscured is not encrypted. What
// rclone declares a password or sensitive counts whatever its name, so
// crypt's password2 and azureblob's sas_url stay hidden too.
func (p *provider) secret(name string) bool {
	o, ok := p.option(name)
	if !ok {
		return secretName(name)
	}
	return o.IsPassword || secretName(name) || (o.Sensitive && !sensitiveButShown[name])
}

// secretWords catch by name the credentials rclone does not declare, such
// as those of a backend this rclone lacks. A name that is or ends in one is
// secret, which leaves out access_key_id, the public half of an S3 pair.
var secretWords = []string{"pass", "password", "secret", "key", "token", "credentials", "passphrase"}

func secretName(name string) bool {
	// key_pem is an SSH private key pasted inline.
	if name == "key_pem" || name == "auth_token" || strings.Contains(name, "secret") {
		return true
	}
	for _, w := range secretWords {
		if name == w || strings.HasSuffix(name, "_"+w) {
			return true
		}
	}
	return false
}

// sensitiveButShown are options rclone marks sensitive, so that a shared
// config leaves them out, which only say which server or which account a
// remote is. Every other sensitive option is withheld, so a credential
// rclone adds later stays hidden without anybody listing it here.
var sensitiveButShown = map[string]bool{
	"host": true, "url": true, "api_url": true, "namenode": true,
	"user": true, "username": true, "email": true, "apple_id": true,
	"account": true, "access_key_id": true, "client_id": true, "app_id": true, "drive_id": true,
	"tenant": true, "tenant_id": true, "tenant_domain": true, "domain": true, "user_id": true,
	"namespace": true, "compartment": true, "project_number": true, "user_project": true,
	"cloud_name": true,
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
	Name     string `json:"name"`
	Help     string `json:"help"`
	Type     string `json:"type"`
	Default  string `json:"default,omitempty"`
	Provider string `json:"provider,omitempty"`
	Required bool   `json:"required"`
	// Essential means the remote will not work in practice without it.
	// rclone's Required follows its own wizard, which prompts for the rest
	// with a default: s3 marks nothing required and smb only the host.
	Essential bool      `json:"essential"`
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
				Name: o.Name, Help: firstLine(o.Help), Type: o.Type, Default: o.DefaultStr, Provider: o.Provider,
				Required: o.Required, Essential: essential[p.Name][o.Name], Secret: p.secret(o.Name), Advanced: o.Advanced, Exclusive: o.Exclusive,
			}
			// Somebody might take an example password literally.
			if !opt.Secret {
				for _, e := range o.Examples {
					opt.Examples = append(opt.Examples, Example{Value: e.Value, Help: firstLine(e.Help), Provider: e.Provider})
				}
			}
			t.Options = append(t.Options, opt)
		}
		slices.SortStableFunc(t.Options, func(a, b Option) int { return cmp.Compare(optionRank(a), optionRank(b)) })
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

// optionRank puts what the form asks first: required and essential options,
// then the ordinary ones, then the advanced ones.
func optionRank(o Option) int {
	switch {
	case o.Required, o.Essential:
		return 0
	case !o.Advanced:
		return 1
	}
	return 2
}

// firstLine keeps the first line of a help text, which is all a bubble
// beside a field has room for.
func firstLine(help string) string {
	line, _, _ := strings.Cut(help, "\n")
	return strings.TrimSpace(line)
}

// essential lists per backend the options without which a remote will not
// work. Order does not matter, the form keeps rclone's.
var essential = map[string]map[string]bool{
	// A wrong provider brings signature errors that read like wrong
	// credentials. AWS wants a region and everybody else an endpoint.
	"s3": {"provider": true, "access_key_id": true, "secret_access_key": true, "region": true, "endpoint": true},
	// A blank domain is right at home and wrong in an office, and a share
	// reached from outside often sits behind a port other than 445.
	"smb": {"host": true, "user": true, "pass": true, "domain": true, "port": true},
	// A password and a key file are alternatives, so both are shown.
	"sftp":   {"host": true, "user": true, "port": true, "pass": true, "key_file": true},
	"crypt":  {"remote": true, "password": true},
	"webdav": {"url": true, "vendor": true, "user": true, "pass": true},
	"mega":   {"user": true, "pass": true, "2fa": true},

	"opendrive":   {"username": true, "password": true},
	"protondrive": {"username": true, "password": true, "2fa": true},
	"seafile":     {"url": true, "user": true, "pass": true, "library": true},
	"koofr":       {"provider": true, "user": true, "password": true},
	// Apple lets rclone in only with the session of a sign-in confirmed by
	// a code, which rclone config makes and which ends after 30 days.
	"iclouddrive": {"apple_id": true, "password": true, "trust_token": true, "cookies": true},
	// A login over plain FTP travels readable, so the encryption is asked.
	"ftp":       {"host": true, "user": true, "pass": true, "port": true, "tls": true, "explicit_tls": true},
	"b2":        {"account": true, "key": true},
	"azureblob": {"account": true, "key": true},
	"storj":     {"provider": true, "access_grant": true},
	"pcloud":    {"hostname": true},
	"drive":     {"scope": true},

	"filescom":        {"site": true, "username": true, "password": true},
	"internetarchive": {"access_key_id": true, "secret_access_key": true},
	"sugarsync":       {"app_id": true, "access_key_id": true, "private_access_key": true},
	"ulozto":          {"username": true, "password": true, "app_token": true},
	"sia":             {"api_url": true, "api_password": true},
	"swift":           {"auth": true, "user": true, "key": true},
	"qingstor":        {"access_key_id": true, "secret_access_key": true, "zone": true},
	"azurefiles":      {"account": true, "key": true, "share_name": true},
	"drime":           {"access_token": true},
	"gofile":          {"access_token": true},
	"fichier":         {"api_key": true},
	"archive":         {"remote": true},

	// OAuth backends without a shared client: the pair is what somebody
	// registers with the provider to get a token at all.
	"hidrive":      {"client_id": true, "client_secret": true},
	"huaweidrive":  {"client_id": true, "client_secret": true},
	"putio":        {"client_id": true, "client_secret": true},
	"sharefile":    {"endpoint": true, "client_id": true, "client_secret": true},
	"zoho":         {"client_id": true, "client_secret": true, "region": true},
	"premiumizeme": {"api_key": true},

	"google cloud storage": {"project_number": true, "service_account_file": true, "client_id": true, "client_secret": true},
	"google photos":        {"client_id": true, "client_secret": true, "read_only": true},
}
