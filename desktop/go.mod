module github.com/junkerderprovinz/cannoncommander/desktop

go 1.27.0

toolchain go1.27.4

// The server is the module next door rather than a published dependency, so
// the window and the container always run the same server.
replace github.com/junkerderprovinz/cannoncommander/backend => ../backend

require (
	github.com/bodgit/sevenzip v1.6.5
	github.com/godbus/dbus/v5 v5.2.2
	github.com/junkerderprovinz/cannoncommander/backend v0.0.0
	github.com/wailsapp/wails/v3 v3.1.0
	golang.org/x/image v0.47.0
	golang.org/x/sys v0.50.0
)

require (
	github.com/alecthomas/kong v1.17.0 // indirect
	github.com/andybalholm/brotli v1.2.8 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/boombuler/barcode v1.1.0 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/coreos/go-oidc/v3 v3.21.0 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/creack/pty v1.1.24 // indirect
	github.com/dhowden/tag v0.0.0-20240417053706-3d75831295e8 // indirect
	github.com/dsoprea/go-exif/v3 v3.0.1 // indirect
	github.com/dsoprea/go-logging v0.0.0-20200710184922-b02d349568dd // indirect
	github.com/dsoprea/go-utility/v2 v2.0.0-20221003172846-a3e1774ef349 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/ebitengine/purego v0.11.3 // indirect
	github.com/evanoberholster/imagemeta v1.1.0 // indirect
	github.com/fxamacker/cbor/v2 v2.10.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.16 // indirect
	github.com/gen2brain/go-fitz v1.28.2 // indirect
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.9 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-playground/locales v0.14.2 // indirect
	github.com/go-playground/universal-translator v0.18.2 // indirect
	github.com/go-playground/validator/v10 v10.30.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/go-webauthn/webauthn v0.18.2 // indirect
	github.com/go-webauthn/x v0.3.1 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang/geo v0.0.0-20261029132150-f51d36688936 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gtsteffaniak/go-cache v1.1.0 // indirect
	github.com/gtsteffaniak/go-ffmpeg v1.6.2 // indirect
	github.com/gtsteffaniak/go-logger v1.1.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/klauspost/cpuid/v2 v2.4.2 // indirect
	github.com/kovidgoyal/go-parallel v1.1.1 // indirect
	github.com/kovidgoyal/go-shm v1.0.0 // indirect
	github.com/kovidgoyal/imaging v1.9.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/leodido/go-urn v1.5.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mattn/go-sqlite3 v1.14.53 // indirect
	github.com/modelcontextprotocol/go-sdk v1.8.0 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/nwaples/rardecode/v2 v2.4.6 // indirect
	github.com/philhofer/fwd v1.2.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.33 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pquerna/otp v1.5.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rwcarlsen/goexif v0.0.0-20190401172101-9e8deecbddbd // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/stangelandcl/ppmd v0.1.1 // indirect
	github.com/tinylib/msgp v1.6.5 // indirect
	github.com/ulikunitz/xz v0.5.17 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yeka/zip v0.0.0-20231116150916-03d6312748a9 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	go4.org v0.0.0-20261028031214-ed6929a230fe // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/oauth2 v0.37.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/term v0.47.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.4.1 // indirect
	modernc.org/libc v1.80.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.61.1 // indirect
)
