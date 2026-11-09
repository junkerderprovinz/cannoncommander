package settings

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

type AllowedMethods string

var Env Environment

const (
	NoAuth       AllowedMethods = "noAuth"
	PasswordAuth AllowedMethods = "passwordAuth"
)

type Settings struct {
	Server       Server       `json:"server"`
	Auth         Auth         `json:"auth"`
	Frontend     Frontend     `json:"frontend"`
	Integrations Integrations `json:"integrations"`
	Http         Http         `json:"http"`
}

type Http struct {
	Socket            string   `json:"socket"`            // socket to listen on - eg. /var/run/cannoncommander.sock
	TLSKey            string   `json:"tlsKey"`            // path to TLS key
	TLSCert           string   `json:"tlsCert"`           // path to TLS cert
	Port              int      `json:"port"`              // port to listen on
	ListenAddress     string   `json:"listen"`            // address to listen on (default: 0.0.0.0)
	BaseURL           string   `json:"baseURL"`           // base URL for the server, the subpath that the server is running on.
	ExternalUrl       string   `json:"externalUrl"`       // used by share links if set (eg. http://mydomain.com)
	InternalUrl       string   `json:"internalUrl"`       // used by integrations if set, this is the base domain that an integration service will use to communicate with the server (eg. http://localhost:8080)
	DisableWebDAV     bool     `json:"disableWebDAV"`     // keeps the WebDAV server off until an admin switches it on in the settings (default: true)
	TrustProxyHeaders bool     `json:"trustProxyHeaders"` // honor X-Forwarded-* and X-Real-IP from a reverse proxy (default: false)
	TrustedProxies    []string `json:"trustedProxies"`    // IPs/CIDRs of trusted proxies; forwarded client headers are only honored when the socket peer matches one of these (default: empty = forwarded headers ignored)
	// UnixSocketTrustedUIDs and UnixSocketTrustedGIDs list peer credentials (Linux SO_PEERCRED) allowed to
	// present X-Forwarded-* on a filesystem http.socket listener. When both are empty, forwarded headers
	// are not trusted on Unix sockets. Abstract sockets (@name) never trust forwarded headers.
	UnixSocketTrustedUIDs []int  `json:"unixSocketTrustedUIDs"`
	UnixSocketTrustedGIDs []int  `json:"unixSocketTrustedGIDs"`
	SocketFileMode        string `json:"socketFileMode"`   // octal mode for filesystem http.socket (default "0660"); ignored for abstract sockets
	SocketGroup           string `json:"socketGroup"`      // optional group name or numeric gid for the socket file (filesystem sockets only)
	DisableRateLimit      bool   `json:"disableRateLimit"` // turns off built-in auth route rate limiting and failed-login lockout (default false).
}

type Environment struct {
	IsCLIMode   bool `json:"-"` // true when running a CLI subcommand (user set, promote, etc.)
	IsDevMode   bool `json:"-"`
	IsFirstLoad bool `json:"-"` // used internally to track if this is the first load of the application
	// ConfigSourceDefaultPermissions maps permission flags explicitly set in config defaultPermissions (e.g. view -> true).
	ConfigSourceDefaultPermissions map[string]bool `json:"-"`
	MuPdfAvailable                 bool            `json:"-"` // used internally if compiled with mupdf support
	EmbeddedFs                     bool            `json:"-"` // used internally if compiled with embedded fs support
}

type Server struct {
	SearchResultsLimit           int            `json:"searchResultsLimit" yaml:"searchResultsLimit" validate:"gte=1"` // maximum requested search results across all sources (default: 1000); quick search defaults to 100, advanced search to 500; size viewer remains limited to 200
	MinSearchLength              int            `json:"minSearchLength" yaml:"minSearchLength"`                        // minimum length of search query to begin searching (default: 3)
	DisableUpdateCheck           bool           `json:"disableUpdateCheck"`                                            // starts the automatic update check on the Info page switched off
	NumImageProcessors           int            `json:"numImageProcessors"`                                            // number of concurrent image processing jobs used to create previews, default is 4.
	DisablePreviews              bool           `json:"disablePreviews"`                                               // disable all previews thumbnails, simple icons will be used
	DisableResize                bool           `json:"disablePreviewResize"`                                          // disable resizing of previews for faster loading over slow connections
	DisableTypeDetectionByHeader bool           `json:"disableTypeDetectionByHeader"`                                  // disable type detection by header, useful if filesystem is slow.
	Logging                      []LogConfig    `json:"logging" yaml:"logging"`
	Sources                      []*Source      `json:"sources" validate:"required,dive"`
	CacheDir                     string         `json:"cacheDir"`        // path to the cache directory, used for thumbnails and other cached files
	CacheDirCleanup              bool           `json:"cacheDirCleanup"` // whether to automatically cleanup the cache directory. Note: docker must also mount a persistent volume to persist the cache (default: false)
	MaxArchiveSizeGB             int64          `json:"maxArchiveSize"`  // maximum archive/unarchive size in GB. 0 means no limit. (default: 20)
	Filesystem                   Filesystem     `json:"filesystem"`      // filesystem settings
	IndexSqlConfig               IndexSqlConfig `json:"indexSqlConfig"`  // Index database SQL configuration
	// not exposed to config
	SourceMap     map[string]*Source   `json:"-" validate:"omitempty"` // uses realpath as key
	DefaultScopes []users.BackendScope `json:"-" validate:"omitempty"` // the defaultEnabled sources
	NameToSource  map[string]*Source   `json:"-" validate:"omitempty"` // uses name as key
	DatabaseV2    Database             `json:"database"`               // SQLite database configuration
}

type ActivityConfig struct {
	Disabled             bool `json:"disabled"`             // disable semantic activity audit logging (default: false)
	RetentionDays        int  `json:"retentionDays"`        // purge activity rows older than this many days (default 30)
	FlushIntervalSeconds int  `json:"flushIntervalSeconds"` // buffer flush interval in seconds (default 10)
	MaxBufferSize        int  `json:"maxBufferSize"`        // max in-memory buffer before immediate flush (default 10000)
}

type Database struct {
	Path     string         `json:"path"`     // path to SQLite database file
	Activity ActivityConfig `json:"activity"` // activity audit logging configuration
}

type Filesystem struct {
	CreateFilePermission      string `json:"createFilePermission" validate:"required,file_permission"`      // Unix permissions like 644, 755, 2755 (default: 644)
	CreateDirectoryPermission string `json:"createDirectoryPermission" validate:"required,file_permission"` // Unix permissions like 755, 2755, 1777 (default: 755)
}

// Index SQL startup integrity modes (IndexSqlConfig.StartupIntegrityCheck).
const (
	// IndexStartupIntegrityQuickCheck runs PRAGMA quick_check, which reads the whole index and takes minutes on a large one.
	IndexStartupIntegrityQuickCheck = "quickCheck"
	// IndexStartupIntegrityProbe reads the schema and one row, which is enough for a cache that rebuilds itself (default).
	IndexStartupIntegrityProbe = "probe"
	// IndexStartupIntegrityOff skips startup checks beyond sql.Open Ping; least safe, fastest boot.
	IndexStartupIntegrityOff = "off"
)

type IndexSqlConfig struct {
	BatchSize             int    `json:"batchSize"`                                                                       // number of items to batch in a single transaction, typically 500-5000. higher = faster but could use more memory.
	CacheSizeMB           int    `json:"cacheSizeMB"`                                                                     // size of the SQLite cache in MB
	WalMode               bool   `json:"walMode"`                                                                         // enable the more complex WAL journaling mode. Slower, more memory usage, but better for deployments with constant user activity.
	DisableReuse          bool   `json:"disableReuse"`                                                                    // enable to always create a new indexing database on startup.
	StartupIntegrityCheck string `json:"startupIntegrityCheck,omitempty" validate:"omitempty,oneof=quickCheck probe off"` // the method used to check the integrity of the index database on startup (default: probe)
}

type Integrations struct {
	OnlyOffice OnlyOffice `json:"office" validate:"omitempty"`
	Media      Media      `json:"media" validate:"omitempty"`
}

// onlyoffice secret is stored in the local.json file
// docker exec <containerID> /var/www/onlyoffice/documentserver/npm/json -f /etc/onlyoffice/documentserver/local.json 'services.CoAuthoring.secret.session.string'
type OnlyOffice struct {
	Url         string `json:"url" validate:"required"`    // The URL to the OnlyOffice Document Server, needs to be accessible to the user.
	InternalUrl string `json:"internalUrl"`                // An optional internal address that the server can use to communicate with the OnlyOffice Document Server, could be useful to bypass proxy.
	Secret      string `json:"secret" validate:"required"` // secret: authentication key for OnlyOffice integration
	ViewOnly    bool   `json:"viewOnly"`                   // view only mode for OnlyOffice
}

type Media struct {
	FfmpegPath               string        `json:"ffmpegPath"`               // path to ffmpeg directory with ffmpeg and ffprobe (eg. /usr/local/bin)
	Convert                  FfmpegConvert `json:"convert"`                  // config for ffmpeg conversion settings
	Debug                    bool          `json:"debug"`                    // output ffmpeg stdout for media integration -- careful can produces lots of output!
	ExtractEmbeddedSubtitles bool          `json:"extractEmbeddedSubtitles"` // extract embedded subtitles from media files
	HardwareAcceleration     bool          `json:"hardwareAcceleration"`     // enable hardware acceleration for ffmpeg if available
}

type FfmpegConvert struct {
	ImagePreview map[ImagePreviewType]*bool `json:"imagePreview"` // supported image preview formats. defaults vary by type (see individual type docs)
	VideoPreview map[VideoPreviewType]*bool `json:"videoPreview"` // supported video preview formats. defaults to true for all types unless explicitly disabled.
}

type ImagePreviewType string

const (
	HEICImagePreview ImagePreviewType = "heic"
	JPEGImagePreview ImagePreviewType = "jpeg" // only used as fallback for JPEG formats that can't otherwise be decoded
	//RAWImagePreview  ImagePreviewType = "raw"
)

func (i ImagePreviewType) String() string {
	return string(i)
}

// AllImagePreviewTypes contains all supported image preview types.
var AllImagePreviewTypes = []ImagePreviewType{
	HEICImagePreview,
	JPEGImagePreview,
	//RAWImagePreview,
}

type VideoPreviewType string

const (
	MP4VideoPreview      VideoPreviewType = "mp4"
	WebMVideoPreview     VideoPreviewType = "webm"
	MOVVideoPreview      VideoPreviewType = "mov"
	AVIVideoPreview      VideoPreviewType = "avi"
	MKVVideoPreview      VideoPreviewType = "mkv"
	FLVVideoPreview      VideoPreviewType = "flv"
	WMVVideoPreview      VideoPreviewType = "wmv"
	M4VVideoPreview      VideoPreviewType = "m4v"
	ThreeGPVideoPreview  VideoPreviewType = "3gp"
	ThreeGP2VideoPreview VideoPreviewType = "3g2"
	TSVideoPreview       VideoPreviewType = "ts"
	M2TSVideoPreview     VideoPreviewType = "m2ts"
	VOBVideoPreview      VideoPreviewType = "vob"
	ASFVideoPreview      VideoPreviewType = "asf"
	MPGVideoPreview      VideoPreviewType = "mpg"
	MPEGVideoPreview     VideoPreviewType = "mpeg"
	F4VVideoPreview      VideoPreviewType = "f4v"
	OGVVideoPreview      VideoPreviewType = "ogv"
)

func (v VideoPreviewType) String() string {
	return string(v)
}

// AllVideoPreviewTypes contains all supported video preview types.
var AllVideoPreviewTypes = []VideoPreviewType{
	MP4VideoPreview,
	WebMVideoPreview,
	MOVVideoPreview,
	AVIVideoPreview,
	MKVVideoPreview,
	FLVVideoPreview,
	WMVVideoPreview,
	M4VVideoPreview,
	ThreeGPVideoPreview,
	ThreeGP2VideoPreview,
	TSVideoPreview,
	M2TSVideoPreview,
	VOBVideoPreview,
	ASFVideoPreview,
	MPGVideoPreview,
	MPEGVideoPreview,
	F4VVideoPreview,
	OGVVideoPreview,
}

type LogConfig struct {
	Levels    string `json:"levels" yaml:"levels"`       // separated list of log levels to enable. (eg. "info|warning|error|debug")
	ApiLevels string `json:"apiLevels" yaml:"apiLevels"` // separated list of log levels to enable for the API. (eg. "info|warning|error")
	Output    string `json:"output" yaml:"output"`       // output location. (eg. "stdout" or "path/to/file.log")
	NoColors  bool   `json:"noColors" yaml:"noColors"`   // disable colors in the output
	Json      bool   `json:"json" yaml:"json"`           // output in json format
	Utc       bool   `json:"utc" yaml:"utc"`             // use UTC time in the output instead of local time.
	ApiFilter string `json:"apiFilter" yaml:"apiFilter"` // regex filter that excludes matching full api paths from being logged. (eg. '/user\?id\=self') Defaults to '^/health|^/favicon.ico|^/static|^/public/static'
}
type Source struct {
	Path   string       `json:"path" validate:"required"` // file system path. (Can be relative)
	Name   string       `json:"name"`                     // display name
	Config SourceConfig `json:"config,omitempty"`
}

type SourceConfig struct {
	DenyByDefault    bool              `json:"denyByDefault,omitempty"` // deprecated: the first start with the app's access rules turns it into a rule that hides the source from everyone
	Private          bool              `json:"private"`                 // designate as source as private -- currently just means no sharing permitted.
	ReadOnly         bool              `json:"readOnly,omitempty"`      // read-only source, changes from the UI, WebDAV and the API are refused.
	Disabled         bool              `json:"disabled,omitempty"`      // disable the source, this is useful so you don't need to remove it from the config file
	Rules            []ConditionalRule `json:"rules"`                   // list of item rules to apply to specific paths
	DefaultUserScope string            `json:"defaultUserScope"`        // defaults to root of index "/" should match folders under path
	DefaultEnabled   bool              `json:"defaultEnabled"`          // should be added as a default source for new users?
	CreateUserDir    bool              `json:"createUserDir"`           // create a user directory for each user under defaultUserScope + username
	UseLogicalSize   bool              `json:"useLogicalSize"`          // calculate sizes based on logical size instead of disk utilization (du -sh), folders will be 0 bytes when empty.
	// DefaultPermissions is the template for new user scopes on this source (also synced globally via Access settings).
	DefaultPermissions users.SourceFilePermissions `json:"defaultPermissions,omitempty" yaml:"defaultPermissions,omitempty"`
	// DefaultPermissionsFromConfig holds permission flags explicitly set under defaultPermissions in config YAML.
	DefaultPermissionsFromConfig map[string]bool `json:"-"`
	// hidden but used internally - optimized map lookups for conditional rules
	ResolvedRules ResolvedRulesConfig `json:"-"`
}

type ConditionalRule struct {
	NeverWatchPath   string `json:"neverWatchPath"`   // index the folder in the first pass to get included in search, but never re-indexed.
	IncludeRootItem  string `json:"includeRootItem"`  // include only these items at root folder level
	FileStartsWith   string `json:"fileStartsWith"`   // (global) exclude files that start with these prefixes. Eg. "archive-" or "backup-"
	FolderStartsWith string `json:"folderStartsWith"` // (global) exclude folders that start with these prefixes. Eg. "archive-" or "backup-"
	FileEndsWith     string `json:"fileEndsWith"`     // (global) exclude files that end with these suffixes. Eg. ".jpg" or ".txt"
	FolderEndsWith   string `json:"folderEndsWith"`   // (global) exclude folders that end with these suffixes. Eg. ".thumbnails" or ".git"
	FolderPath       string `json:"folderPath"`       // (global) exclude folders that match this path. Eg. "/path/to/folder" or "/path/to/folder/subfolder"
	FilePath         string `json:"filePath"`         // (global) exclude files that match this path. Eg. "/path/to/file.txt" or "/path/to/file.txt/subfile.txt"
	FileName         string `json:"fileName"`         // (global) exclude files that match these names. Eg. "file.txt" or "test.csv"
	FolderName       string `json:"folderName"`       // (global) exclude folders that match these names. Eg. "folder" or "subfolder"

	Viewable              bool `json:"viewable"`              // Enable viewing in UI but exclude from indexing
	IgnoreHidden          bool `json:"ignoreHidden"`          // Excludes only hidden files and folders
	IgnoreZeroSizeFolders bool `json:"ignoreZeroSizeFolders"` // Excludes only folders with 0 size
	IgnoreSymlinks        bool `json:"ignoreSymlinks"`        // Excludes symbolic links
}

// ResolvedRulesConfig provides O(1) lookup performance for conditional rules.
// Maps are built from Rules during initialization.
type ResolvedRulesConfig struct {
	// Exact match maps - O(1) lookup (only for names, not StartsWith/EndsWith)
	FileNames   map[string]ConditionalRule // key: file name
	FolderNames map[string]ConditionalRule // key: folder name

	// exact match for paths
	FilePaths   map[string]ConditionalRule // key: file path
	FolderPaths map[string]ConditionalRule // key: folder path

	FileEndsWith     []ConditionalRule // list of item rules that have been resolved for specific paths
	FolderEndsWith   []ConditionalRule // list of item rules that have been resolved for specific paths
	FileStartsWith   []ConditionalRule // list of item rules that have been resolved for specific paths
	FolderStartsWith []ConditionalRule // list of item rules that have been resolved for specific paths

	// NeverWatch paths map - O(1) lookup for all paths with neverWatch: true
	// This replaces the old NeverWatchPaths slice
	NeverWatchPaths  map[string]struct{} // key: full folder path
	IncludeRootItems map[string]struct{} // key: inclusive root item

	IgnoreAllHidden          bool // Excludes all hidden files and folders
	IgnoreAllZeroSizeFolders bool // Excludes all folders with 0 size
	IgnoreAllSymlinks        bool // Excludes all symbolic links
	IndexingDisabled         bool // Excludes all files and folders from indexing
	NoRules                  bool // No rules are configured
}

type Frontend struct {
	Name                  string `json:"name"`                  // the name the passkey prompt shows for this server
	DisableUsedPercentage bool   `json:"disableUsedPercentage"` // skip the used space of the sources
	OIDCLoginButtonText   string `json:"oidcLoginButtonText"`   // text on the single sign on button of the sign in page
}
