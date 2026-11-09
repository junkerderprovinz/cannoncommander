package remotes

import (
	"cmp"
	"slices"
	"strings"
)

// Provider is a product somebody looks for, as opposed to the rclone backend
// that reaches it: Nextcloud, ownCloud and OpenCloud are three products and
// one webdav backend. A backend without an entry is still offered after the
// list.
type Provider struct {
	// ID is what the interface sends back, since display names are
	// translated.
	ID string `json:"id"`
	// Name is the product's own and is not translated.
	Name    string `json:"name"`
	Backend string `json:"backend"`
	Group   Group  `json:"group"`
	// Preset is written into the remote without asking, such as the vendor
	// that makes WebDAV work properly against a self-hosted cloud.
	Preset map[string]string `json:"preset,omitempty"`
	// Mark names the product's logo in the interface, or a glyph where a
	// logo would name the wrong service.
	Mark string `json:"mark,omitempty"`
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

// Group is the heading a provider stands under in the picker.
//
// A service that hands out buckets is storage whoever runs it, so MinIO and
// Ceph are storage although they ask for an endpoint. Of the rest, what
// somebody signs into by name is a cloud and what they point at by address
// is a protocol. Nextcloud and Seafile ask for an address but are clouds,
// because people look for them by name.
type Group string

const (
	GroupCloud    Group = "cloud"
	GroupStorage  Group = "storage"
	GroupProtocol Group = "protocol"
)

// providers is kept by hand, since rclone does not know what products exist
// or what they are called. A product wears its own mark or its owner's,
// never a neighbour's.
var providers = []Provider{
	{ID: "nextcloud", Name: "Nextcloud", Auth: AuthAppPassword, Backend: "webdav", Group: GroupCloud,
		Preset: map[string]string{"vendor": "nextcloud"}, Mark: "IconNextcloud",
		URLHint: "https://cloud.example.com/remote.php/webdav/"},
	{ID: "owncloud", Name: "ownCloud", Auth: AuthAppPassword, Backend: "webdav", Group: GroupCloud,
		Preset: map[string]string{"vendor": "owncloud"}, Mark: "IconOwncloud",
		URLHint: "https://cloud.example.com/remote.php/webdav/"},
	// OpenCloud is a fork of ownCloud Infinite Scale, whose vendor speaks
	// another dialect than ownCloud 10's.
	{ID: "opencloud", Name: "OpenCloud", Auth: AuthAppPassword, Backend: "webdav", Group: GroupCloud,
		Preset: map[string]string{"vendor": "infinitescale"}, Mark: "IconOpencloud",
		URLHint: "https://cloud.example.com/remote.php/webdav"},
	// rclone has no OxiCloud vendor, so the description is what tells its
	// remotes from any other WebDAV server.
	{ID: "oxicloud", Name: "OxiCloud", Auth: AuthAppPassword, Backend: "webdav", Group: GroupCloud,
		Preset: map[string]string{"vendor": "other", "description": "OxiCloud"}, Mark: "IconTabStorage",
		URLHint: "https://cloud.example.com/webdav/"},

	{ID: "dropbox", Name: "Dropbox", Backend: "dropbox", Group: GroupCloud, Mark: "IconDropbox"},
	{ID: "gdrive", Name: "Google Drive", Backend: "drive", Group: GroupCloud, Mark: "IconGoogleDrive"},
	{ID: "onedrive", Name: "OneDrive", Backend: "onedrive", Group: GroupCloud, Mark: "IconOnedrive"},
	{ID: "mega", Name: "MEGA", Backend: "mega", Group: GroupCloud, Mark: "IconMega"},
	{ID: "pcloud", Name: "pCloud", Backend: "pcloud", Group: GroupCloud, Mark: "IconPcloud"},
	{ID: "box", Name: "Box", Backend: "box", Group: GroupCloud, Mark: "IconBox"},
	{ID: "protondrive", Name: "Proton Drive", Backend: "protondrive", Group: GroupCloud, Mark: "IconProtonDrive"},
	// Apple refuses app passwords here: the Apple ID password goes with a
	// trust token from rclone config.
	{ID: "icloud", Name: "iCloud Drive", Auth: AuthLogin, Backend: "iclouddrive", Group: GroupCloud, Mark: "IconICloud"},
	{ID: "jottacloud", Name: "Jottacloud", Auth: AuthLoginToken, AuthURL: "https://www.jottacloud.com/web/secure",
		Backend: "jottacloud", Group: GroupCloud, Mark: "IconJottacloud"},
	// Koofr's fields look like an ordinary login, but the password is made
	// in the service's settings.
	{ID: "koofr", Name: "Koofr", Auth: AuthAppPassword, Backend: "koofr", Group: GroupCloud, Mark: "IconKoofr"},
	{ID: "seafile", Name: "Seafile", Auth: AuthAppPassword, Backend: "seafile", Group: GroupCloud, Mark: "IconSeafile"},
	{ID: "opendrive", Name: "OpenDrive", Backend: "opendrive", Group: GroupCloud, Mark: "IconOpendrive"},
	{ID: "yandex", Name: "Yandex Disk", Backend: "yandex", Group: GroupCloud, Mark: "IconYandex"},
	{ID: "mailru", Name: "Mail.ru Cloud", Backend: "mailru", Group: GroupCloud, Mark: "IconMailru"},
	{ID: "zoho", Name: "Zoho WorkDrive", Backend: "zoho", Group: GroupCloud, Mark: "IconZoho"},
	{ID: "hidrive", Name: "IONOS HiDrive", Backend: "hidrive", Group: GroupCloud, Mark: "IconIonos"},
	{ID: "sharefile", Name: "ShareFile", Backend: "sharefile", Group: GroupCloud, Mark: "IconCitrix"},
	{ID: "sugarsync", Name: "SugarSync", Backend: "sugarsync", Group: GroupCloud, Mark: "IconSugarsync"},
	{ID: "putio", Name: "put.io", Backend: "putio", Group: GroupCloud, Mark: "IconPutio"},
	{ID: "pikpak", Name: "PikPak", Backend: "pikpak", Group: GroupCloud, Mark: "IconPikpak"},
	{ID: "internxt", Name: "Internxt", Backend: "internxt", Group: GroupCloud, Mark: "IconInternxt"},
	{ID: "filen", Name: "Filen", Backend: "filen", Group: GroupCloud, Mark: "IconFilen"},
	{ID: "filescom", Name: "Files.com", Backend: "filescom", Group: GroupCloud, Mark: "IconFilesCom"},
	{ID: "huaweidrive", Name: "Huawei Drive", Backend: "huaweidrive", Group: GroupCloud, Mark: "IconHuaweiCloud"},
	{ID: "ulozto", Name: "Ulož.to", Backend: "ulozto", Group: GroupCloud, Mark: "IconUlozto"},
	{ID: "quatrix", Name: "Quatrix", Backend: "quatrix", Group: GroupCloud, Mark: "IconQuatrix"},
	{ID: "linkbox", Name: "Linkbox", Backend: "linkbox", Group: GroupCloud, Mark: "IconLinkbox"},
	{ID: "gofile", Name: "Gofile", Backend: "gofile", Group: GroupCloud, Mark: "IconGofile"},
	{ID: "pixeldrain", Name: "Pixeldrain", Backend: "pixeldrain", Group: GroupCloud, Mark: "IconPixeldrain"},
	{ID: "googlephotos", Name: "Google Photos", Backend: "google photos", Group: GroupCloud, Mark: "IconGooglePhotos"},
	{ID: "premiumize", Name: "premiumize.me", Backend: "premiumizeme", Group: GroupCloud, Mark: "IconPremiumize"},
	{ID: "fichier", Name: "1Fichier", Auth: AuthAPIKey, AuthURL: "https://1fichier.com/console/params.pl",
		Backend: "fichier", Group: GroupCloud, Mark: "IconFichier"},
	{ID: "drime", Name: "Drime", Auth: AuthAPIKey, Backend: "drime", Group: GroupCloud, Mark: "IconDrime"},
	{ID: "filelu", Name: "FileLu", Auth: AuthAPIKey, Backend: "filelu", Group: GroupCloud, Mark: "IconFilelu"},
	{ID: "shade", Name: "Shade", Auth: AuthAPIKey, Backend: "shade", Group: GroupCloud, Mark: "IconShade"},
	// The product's own logo is gone and Nasuni's names its owner, not the
	// product.
	{ID: "filefabric", Name: "Enterprise File Fabric", Backend: "filefabric", Group: GroupCloud, Mark: "IconTabStorage",
		URLHint: "https://yourfabric.smestorage.com"},

	{ID: "b2", Name: "Backblaze B2", Backend: "b2", Group: GroupStorage, Mark: "IconBackblaze"},
	{ID: "azureblob", Name: "Azure Blob Storage", Backend: "azureblob", Group: GroupStorage, Mark: "IconAzure"},
	{ID: "gcs", Name: "Google Cloud Storage", Backend: "google cloud storage", Group: GroupStorage, Mark: "IconGoogleCloud"},
	{ID: "huaweiobs", Name: "Huawei Cloud OBS", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "HuaweiOBS"}, Mark: "IconHuaweiCloud"},

	// S3-compatible services by name, so nobody has to know to pick S3. The
	// presets are rclone's own spelling, since rclone treats an unknown
	// provider as plain S3 without a word.
	{ID: "wasabi", Name: "Wasabi", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "Wasabi"}, Mark: "IconWasabi"},
	{ID: "r2", Name: "Cloudflare R2", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "Cloudflare"}, Mark: "IconCloudflare"},
	{ID: "spaces", Name: "DigitalOcean Spaces", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "DigitalOcean"}, Mark: "IconDigitalOcean"},
	{ID: "idrivee2", Name: "IDrive e2", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "IDrive"}, Mark: "IconIdrive"},
	{ID: "scaleway", Name: "Scaleway Object Storage", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "Scaleway"}, Mark: "IconScaleway"},
	{ID: "hetznerobj", Name: "Hetzner Object Storage", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "Hetzner"}, Mark: "IconHetzner"},
	{ID: "ionosobj", Name: "IONOS Object Storage", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "IONOS"}, Mark: "IconIonos"},
	{ID: "linode", Name: "Linode Object Storage", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "Linode"}, Mark: "IconLinode"},
	{ID: "ovh", Name: "OVHcloud Object Storage", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "OVHcloud"}, Mark: "IconOvh"},
	{ID: "synologyc2", Name: "Synology C2", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "Synology"}, Mark: "IconSynology"},
	{ID: "oracle", Name: "Oracle Object Storage", Backend: "oracleobjectstorage", Group: GroupStorage, Mark: "IconOracleCloud"},
	{ID: "storj", Name: "Storj", Backend: "storj", Group: GroupStorage, Mark: "IconStorj"},
	{ID: "swift", Name: "OpenStack Swift", Backend: "swift", Group: GroupStorage, Mark: "IconOpenstack"},
	{ID: "netstorage", Name: "Akamai NetStorage", Backend: "netstorage", Group: GroupStorage, Mark: "IconAkamai"},
	{ID: "cloudinary", Name: "Cloudinary", Backend: "cloudinary", Group: GroupStorage, Mark: "IconCloudinary"},
	{ID: "internetarchive", Name: "Internet Archive", Backend: "internetarchive", Group: GroupStorage, Mark: "IconInternetArchive"},
	{ID: "azurefiles", Name: "Azure Files", Backend: "azurefiles", Group: GroupStorage, Mark: "IconAzure"},
	{ID: "qingstor", Name: "QingStor", Backend: "qingstor", Group: GroupStorage, Mark: "IconQingcloud"},
	{ID: "imagekit", Name: "ImageKit", Auth: AuthAPIKey, AuthURL: "https://imagekit.io/dashboard/developer/api-keys",
		Backend: "imagekit", Group: GroupStorage, Mark: "IconImagekit"},
	{ID: "sia", Name: "Sia", Backend: "sia", Group: GroupStorage, Mark: "IconSia"},
	{ID: "minio", Name: "MinIO", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "Minio"}, Mark: "IconMinio"},
	{ID: "seaweedfs", Name: "SeaweedFS", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "SeaweedFS"}, Mark: "IconSeaweedfs"},
	{ID: "ceph", Name: "Ceph", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "Ceph"}, Mark: "IconCeph"},
	// Garage signs for its s3_region only, which is garage unless
	// garage.toml names another.
	{ID: "garage", Name: "Garage", Backend: "s3", Group: GroupStorage,
		Preset: map[string]string{"provider": "Other"}, Mark: "IconGarage",
		Defaults: map[string]string{"region": "garage"}},
	{ID: "s3", Name: "S3 compatible", Backend: "s3", Group: GroupStorage, Mark: "IconDatabase"},

	{ID: "smb", Name: "SMB / Windows share", Backend: "smb", Group: GroupProtocol, Mark: "IconFolder"},
	{ID: "sftp", Name: "SFTP", Backend: "sftp", Group: GroupProtocol, Mark: "IconNetwork"},
	{ID: "webdav", Name: "WebDAV", Auth: AuthAppPassword, Backend: "webdav", Group: GroupProtocol,
		Mark: "IconBrowser", URLHint: "https://server.example.com/remote.php/webdav/"},
	{ID: "ftp", Name: "FTP", Backend: "ftp", Group: GroupProtocol, Mark: "IconUpload"},
	{ID: "http", Name: "HTTP", Backend: "http", Group: GroupProtocol, Mark: "IconLink"},
	{ID: "hdfs", Name: "HDFS", Backend: "hdfs", Group: GroupProtocol, Mark: "IconHadoop"},
	{ID: "doi", Name: "DOI", Backend: "doi", Group: GroupProtocol, Mark: "IconDoi"},

	// Remotes built on another remote. Their names are words, translated in
	// the interface.
	{ID: "crypt", Name: "Encrypted", Backend: "crypt", Group: GroupProtocol, Mark: "IconShieldOn"},
	{ID: "alias", Name: "Alias", Backend: "alias", Group: GroupProtocol, Mark: "IconLink"},
	{ID: "archive", Name: "Archive files", Backend: "archive", Group: GroupProtocol, Mark: "IconArchive"},
	{ID: "cache", Name: "Cache", Backend: "cache", Group: GroupProtocol, Mark: "IconRefresh"},
	{ID: "chunker", Name: "Split into pieces", Backend: "chunker", Group: GroupProtocol, Mark: "IconCopy"},
	{ID: "combine", Name: "Combined remotes", Backend: "combine", Group: GroupProtocol, Mark: "IconAdd"},
	{ID: "compress", Name: "Compressed", Backend: "compress", Group: GroupProtocol, Mark: "IconDownload"},
	{ID: "hasher", Name: "Checksums", Backend: "hasher", Group: GroupProtocol, Mark: "IconCheck"},
	{ID: "union", Name: "Merged remotes", Backend: "union", Group: GroupProtocol, Mark: "IconSync"},
	{ID: "memory", Name: "In memory", Backend: "memory", Group: GroupProtocol, Mark: "IconDiagnostics"},
}

// formerNames maps a backend rclone still reads under an older name to the
// one whose entry stands for both.
var formerNames = map[string]string{"tardigrade": "storj"}

func groupOrder(g Group) int {
	switch g {
	case GroupCloud:
		return 0
	case GroupStorage:
		return 1
	}
	return 2
}

// providersFor lists the providers whose backend this rclone has, by group
// and then by name, and the backends no entry covers.
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
	slices.SortStableFunc(out, func(a, b Provider) int {
		return cmp.Or(cmp.Compare(groupOrder(a.Group), groupOrder(b.Group)), strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)))
	})
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
