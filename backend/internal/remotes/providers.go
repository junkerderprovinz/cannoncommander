package remotes

import (
	"slices"
	"strings"
)

// Provider is what connecting to a product of GlimStone's storage catalog
// takes, by the catalog's id. The catalog holds the names, groups and logos
// the picker shows. Nextcloud, ownCloud and OpenCloud are three products and
// one webdav backend. A backend without an entry is still offered after the
// products.
type Provider struct {
	// ID is the catalog's and what the interface sends back.
	ID      string `json:"id"`
	Backend string `json:"backend"`
	// Preset is written into the remote without asking, such as the vendor
	// that makes WebDAV work properly against a self-hosted cloud.
	Preset map[string]string `json:"preset,omitempty"`
	// URLHint is the shape of the product's address where it is not the one
	// people have in their browser.
	URLHint string    `json:"urlHint,omitempty"`
	Auth    AuthStyle `json:"auth,omitempty"`
	// AuthURL is the page where the credential is made, where there is one
	// shared page to point at.
	AuthURL string `json:"authUrl,omitempty"`
	// Defaults are what the product's own server assumes for a field left
	// empty, shown in that field.
	Defaults map[string]string `json:"defaults,omitempty"`
}

// AuthStyle is how a product wants to be signed into. The set is closed,
// because each value has a sentence in every language of the interface.
type AuthStyle string

const (
	// AuthAppPassword is a password made in the account's security settings,
	// which works where the login password stops at two-factor sign-in.
	AuthAppPassword AuthStyle = "apppassword"
	AuthOAuth       AuthStyle = "oauth"
	// AuthAPIKey is a key and secret made in the service's own console.
	AuthAPIKey AuthStyle = "apikey"
	// AuthAccessKey is the S3 pair of a public key id and a secret.
	AuthAccessKey AuthStyle = "accesskey"
	AuthLogin     AuthStyle = "login"
	// AuthLoginToken is a personal login token that rclone's setup trades
	// once for a lasting sign-in.
	AuthLoginToken AuthStyle = "logintoken"
)

// providers is kept by hand, since rclone does not know what products exist
// or which of its settings each one needs.
var providers = []Provider{
	{ID: "nextcloud", Auth: AuthAppPassword, Backend: "webdav",
		Preset:  map[string]string{"vendor": "nextcloud"},
		URLHint: "https://cloud.example.com/remote.php/webdav/"},
	{ID: "owncloud", Auth: AuthAppPassword, Backend: "webdav",
		Preset:  map[string]string{"vendor": "owncloud"},
		URLHint: "https://cloud.example.com/remote.php/webdav/"},
	// OpenCloud is a fork of ownCloud Infinite Scale, whose vendor speaks
	// another dialect than ownCloud 10's.
	{ID: "opencloud", Auth: AuthAppPassword, Backend: "webdav",
		Preset:  map[string]string{"vendor": "infinitescale"},
		URLHint: "https://cloud.example.com/remote.php/webdav"},
	// rclone has no OxiCloud vendor, so the description is what tells its
	// remotes from any other WebDAV server.
	{ID: "oxicloud", Auth: AuthAppPassword, Backend: "webdav",
		Preset:  map[string]string{"vendor": "other", "description": "OxiCloud"},
		URLHint: "https://cloud.example.com/webdav/"},

	{ID: "dropbox", Backend: "dropbox"},
	{ID: "gdrive", Backend: "drive"},
	{ID: "onedrive", Backend: "onedrive"},
	{ID: "mega", Backend: "mega"},
	{ID: "pcloud", Backend: "pcloud"},
	{ID: "box", Backend: "box"},
	{ID: "protondrive", Backend: "protondrive"},
	// Apple refuses app passwords here: the Apple ID password goes with a
	// trust token from rclone config.
	{ID: "icloud", Auth: AuthLogin, Backend: "iclouddrive"},
	{ID: "jottacloud", Auth: AuthLoginToken, AuthURL: "https://www.jottacloud.com/web/secure",
		Backend: "jottacloud"},
	// Koofr's fields look like an ordinary login, but the password is made
	// in the service's settings.
	{ID: "koofr", Auth: AuthAppPassword, Backend: "koofr"},
	{ID: "seafile", Auth: AuthAppPassword, Backend: "seafile"},
	{ID: "opendrive", Backend: "opendrive"},
	{ID: "yandex", Backend: "yandex"},
	{ID: "mailru", Backend: "mailru"},
	{ID: "zoho", Backend: "zoho"},
	{ID: "hidrive", Backend: "hidrive"},
	{ID: "sharefile", Backend: "sharefile"},
	{ID: "sugarsync", Backend: "sugarsync"},
	{ID: "putio", Backend: "putio"},
	{ID: "pikpak", Backend: "pikpak"},
	{ID: "internxt", Backend: "internxt"},
	{ID: "filen", Backend: "filen"},
	{ID: "filescom", Backend: "filescom"},
	{ID: "huaweidrive", Backend: "huaweidrive"},
	{ID: "ulozto", Backend: "ulozto"},
	{ID: "quatrix", Backend: "quatrix"},
	{ID: "linkbox", Backend: "linkbox"},
	{ID: "gofile", Backend: "gofile"},
	{ID: "pixeldrain", Backend: "pixeldrain"},
	{ID: "googlephotos", Backend: "google photos"},
	{ID: "premiumize", Backend: "premiumizeme"},
	{ID: "fichier", Auth: AuthAPIKey, AuthURL: "https://1fichier.com/console/params.pl", Backend: "fichier"},
	{ID: "drime", Auth: AuthAPIKey, Backend: "drime"},
	{ID: "filelu", Auth: AuthAPIKey, Backend: "filelu"},
	{ID: "shade", Auth: AuthAPIKey, Backend: "shade"},
	{ID: "filefabric", Backend: "filefabric", URLHint: "https://yourfabric.smestorage.com"},

	{ID: "b2", Backend: "b2"},
	{ID: "azureblob", Backend: "azureblob"},
	{ID: "gcs", Backend: "google cloud storage"},
	{ID: "huaweiobs", Backend: "s3", Preset: map[string]string{"provider": "HuaweiOBS"}},

	// S3-compatible services by name, so nobody has to know to pick S3. The
	// presets are rclone's own spelling, since rclone treats an unknown
	// provider as plain S3 without a word.
	{ID: "wasabi", Backend: "s3", Preset: map[string]string{"provider": "Wasabi"}},
	{ID: "r2", Backend: "s3", Preset: map[string]string{"provider": "Cloudflare"}},
	{ID: "spaces", Backend: "s3", Preset: map[string]string{"provider": "DigitalOcean"}},
	{ID: "idrivee2", Backend: "s3", Preset: map[string]string{"provider": "IDrive"}},
	{ID: "scaleway", Backend: "s3", Preset: map[string]string{"provider": "Scaleway"}},
	{ID: "hetznerobj", Backend: "s3", Preset: map[string]string{"provider": "Hetzner"}},
	{ID: "ionosobj", Backend: "s3", Preset: map[string]string{"provider": "IONOS"}},
	{ID: "linode", Backend: "s3", Preset: map[string]string{"provider": "Linode"}},
	{ID: "ovh", Backend: "s3", Preset: map[string]string{"provider": "OVHcloud"}},
	{ID: "synologyc2", Backend: "s3", Preset: map[string]string{"provider": "Synology"}},
	{ID: "oracle", Backend: "oracleobjectstorage"},
	{ID: "storj", Backend: "storj"},
	{ID: "swift", Backend: "swift"},
	{ID: "netstorage", Backend: "netstorage"},
	{ID: "cloudinary", Backend: "cloudinary"},
	{ID: "internetarchive", Backend: "internetarchive"},
	{ID: "azurefiles", Backend: "azurefiles"},
	{ID: "qingstor", Backend: "qingstor"},
	{ID: "imagekit", Auth: AuthAPIKey, AuthURL: "https://imagekit.io/dashboard/developer/api-keys",
		Backend: "imagekit"},
	{ID: "sia", Backend: "sia"},
	{ID: "minio", Backend: "s3", Preset: map[string]string{"provider": "Minio"}},
	{ID: "seaweedfs", Backend: "s3", Preset: map[string]string{"provider": "SeaweedFS"}},
	{ID: "ceph", Backend: "s3", Preset: map[string]string{"provider": "Ceph"}},
	// Garage signs for its s3_region only, which is garage unless
	// garage.toml names another.
	{ID: "garage", Backend: "s3", Preset: map[string]string{"provider": "Other"},
		Defaults: map[string]string{"region": "garage"}},
	{ID: "s3", Backend: "s3"},

	{ID: "smb", Backend: "smb"},
	{ID: "sftp", Backend: "sftp"},
	{ID: "webdav", Auth: AuthAppPassword, Backend: "webdav",
		URLHint: "https://server.example.com/remote.php/webdav/"},
	{ID: "ftp", Backend: "ftp"},
	{ID: "http", Backend: "http"},
	{ID: "hdfs", Backend: "hdfs"},
	{ID: "doi", Backend: "doi"},

	// Remotes built on another remote.
	{ID: "crypt", Backend: "crypt"},
	{ID: "alias", Backend: "alias"},
	{ID: "archive", Backend: "archive"},
	{ID: "cache", Backend: "cache"},
	{ID: "chunker", Backend: "chunker"},
	{ID: "combine", Backend: "combine"},
	{ID: "compress", Backend: "compress"},
	{ID: "hasher", Backend: "hasher"},
	{ID: "union", Backend: "union"},
	{ID: "memory", Backend: "memory"},
}

// formerNames maps a backend rclone still reads under an older name to the
// one whose entry stands for both.
var formerNames = map[string]string{"tardigrade": "storj"}

// providersFor lists the providers whose backend this rclone has, by id, and
// the backends no entry covers.
func providersFor(ts []Type) (out []Provider, unlisted []string) {
	have := map[string]Type{}
	for _, t := range ts {
		have[t.ID] = t
	}
	out = []Provider{}
	named := map[string]bool{}
	for old := range formerNames {
		named[old] = true
	}
	for _, p := range providers {
		named[p.Backend] = true
		t, ok := have[p.Backend]
		if !ok {
			continue
		}
		if p.Auth == "" {
			p.Auth = derivedAuth(t)
		}
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b Provider) int { return strings.Compare(a.ID, b.ID) })
	unlisted = []string{}
	for _, t := range ts {
		if !named[t.ID] && t.ID != "local" {
			unlisted = append(unlisted, t.ID)
		}
	}
	slices.Sort(unlisted)
	return out, unlisted
}

// derivedAuth reads how a backend wants to be signed into from the fields it
// asks for, so the sentence follows rclone rather than a table that goes
// stale. Only two unmistakable shapes count, since a wrong sentence sends
// somebody looking in the wrong place.
func derivedAuth(t Type) AuthStyle {
	if t.OAuth {
		return AuthOAuth
	}
	var keyID, keySecret, user, pass, token bool
	for _, o := range t.Options {
		if !o.Required && !o.Essential {
			continue
		}
		switch o.Name {
		case "access_key_id", "account":
			keyID = true
		case "secret_access_key", "key", "secret":
			keySecret = true
		case "user", "username", "apple_id", "email":
			user = true
		case "pass", "password":
			pass = true
		case "token", "app_token", "api_key", "access_token":
			token = true
		}
	}
	switch {
	case keyID && keySecret:
		return AuthAccessKey
	case user && pass && !token:
		return AuthLogin
	}
	return ""
}

// providerFor names the product behind a saved remote, or "" when its
// parameters do not pin one down. Each product writes its preset into the
// remote, and the match with the most preset keys wins, so a MinIO remote
// is MinIO rather than plain S3. A tie, which only a hand-edited file can
// make, names nothing.
func providerFor(typ string, params map[string]string) string {
	if now, ok := formerNames[typ]; ok {
		typ = now
	}
	best, bestKeys, tied := "", -1, false
	for _, p := range providers {
		if p.Backend != typ || !presetHolds(p.Preset, params) {
			continue
		}
		switch {
		case len(p.Preset) > bestKeys:
			best, bestKeys, tied = p.ID, len(p.Preset), false
		case len(p.Preset) == bestKeys:
			tied = true
		}
	}
	if tied {
		return ""
	}
	return best
}

func presetHolds(preset, params map[string]string) bool {
	for k, want := range preset {
		if params[k] != want {
			return false
		}
	}
	return true
}
