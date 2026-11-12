// Builds the desktop app for the platform it runs on, the way CI does.
//
//     node scripts/desktop.mjs [--arch amd64|arm64] [--installer]
//
// Windows gets CannonCommander.exe, macOS CannonCommander.app holding one
// binary for both architectures, and Linux the bare binary, all in
// desktop/build/bin. With --installer, Windows and Linux also get their
// installer in desktop/build/installer, built from desktop/installer.
//
// The interface is built first and embedded from backend/internal/web/embed,
// like the server build. Wails' generated files (manifest, version resource,
// Info.plist, the icons) are written into desktop/build on every run and
// ignored by git, so a build leaves the tree as it found it.
//
// The Mac and Linux builds link WebKit through cgo and render PDF pages with
// MuPDF as the container does, so they need a C compiler: clang on a Mac and
// gcc on Linux. Windows goes without MuPDF: with it linked in, MinGW's linker
// finds two manifests it cannot merge, and the one it drops may be the
// window's, which carries the DPI awareness. Nothing else there uses cgo, so
// Windows builds without a C compiler, for x64 and ARM alike.
//
// On a Mac the bundle also gets the helper programs, which desktop/modules
// puts together from the pinned upstream files. That builds mutool from
// MuPDF's source, with make and clang. Windows and Linux fetch theirs from
// the release instead; desktop.yml makes those zips.

import { execFileSync, spawnSync } from 'node:child_process'
import { cpSync, mkdirSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { basename, dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = dirname(dirname(fileURLToPath(import.meta.url)))
const desktop = join(root, 'desktop')
const bin = join(desktop, 'build', 'bin')

const args = process.argv.slice(2)
const option = (name) => {
  const at = args.indexOf(name)
  return at >= 0 ? args[at + 1] : undefined
}
const hostArch = process.arch === 'arm64' ? 'arm64' : 'amd64'
const arch = option('--arch') ?? hostArch
const installer = args.includes('--installer')

/** Runs git and returns its answer, or an empty string when it has none. */
function git(...gitArgs) {
  try {
    return execFileSync('git', gitArgs, { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim()
  } catch {
    // A source archive has no .git.
    return ''
  }
}

// The tag when the commit has one, git describe's longer form otherwise,
// which the updater reads as a build that is no release. CC_VERSION pins it
// for every step of one CI run.
const stamp = process.env.CC_VERSION || git('describe', '--tags', '--always', '--dirty') || 'dev'
const commit = git('rev-parse', 'HEAD')
// Info.plist and the installer take three numbers only, so an untagged build
// shows 0.0.0 there; the program itself names the exact build.
const tag = (git('describe', '--tags', '--abbrev=0') || '').replace(/^v/, '')
const plainVersion = /^\d+\.\d+\.\d+$/.test(tag) ? tag : '0.0.0'
// Windows' fixed version field holds four 16-bit numbers; the fourth is the
// commit count, so it moves forward between releases.
const count = Math.min(parseInt(git('rev-list', '--count', 'HEAD'), 10) || 0, 65535)
const numericVersion = `${plainVersion}.${count}`

const versionFlags = [
  `-X github.com/junkerderprovinz/cannoncommander/backend/internal/version.Version=${stamp}`,
  commit ? `-X github.com/junkerderprovinz/cannoncommander/backend/internal/version.CommitSHA=${commit}` : '',
].filter(Boolean).join(' ')

function run(command, commandArgs, { cwd = desktop, env = {} } = {}) {
  const res = spawnSync(command, commandArgs, { cwd, stdio: 'inherit', env: { ...process.env, ...env } })
  if (res.status !== 0) {
    console.error(`failed: ${command} ${commandArgs.join(' ')}`)
    process.exit(res.status ?? 1)
  }
}

// The desktop tag compiles in the parts of the server that only the app has.
function goBuild(out, { goos, goarch, tags, cgo = '1', ldflags = '', env = {} }) {
  run('go', [
    'build', '-tags', `desktop,${tags}`, '-trimpath', '-buildvcs=false',
    '-ldflags', `-s -w ${ldflags} ${versionFlags}`,
    '-o', out, '.',
  ], { env: { GOOS: goos, GOARCH: goarch, CGO_ENABLED: cgo, ...env } })
}

// buildInstaller builds desktop/installer around the program at exe, into
// desktop/build/installer so desktop/build/bin stays the program alone. The
// installer itself needs no cgo on Windows; on Linux its window does.
function buildInstaller(exe, name, { goos, cgo, tags, ldflags = '' }) {
  const payload = join(desktop, 'installer', 'payload')
  rmSync(payload, { recursive: true, force: true })
  mkdirSync(payload, { recursive: true })
  cpSync(exe, join(payload, basename(exe)))
  try {
    run('go', [
      'build', '-tags', tags, '-trimpath', '-buildvcs=false',
      '-ldflags', `-s -w ${ldflags} -X main.version=${plainVersion}`,
      '-o', join(desktop, 'build', 'installer', name), '.',
    ], { cwd: join(desktop, 'installer'), env: { GOOS: goos, GOARCH: arch, CGO_ENABLED: cgo } })
  } finally {
    rmSync(payload, { recursive: true, force: true })
  }
}

// The version resource Wails compiles into the executable, written after
// Wails' own, whose strings sit under a language Windows does not read.
function writeWindowsVersion() {
  const info = {
    fixed: { file_version: numericVersion, product_version: numericVersion },
    info: {
      '0409': {
        ProductVersion: stamp,
        CompanyName: 'junkerderprovinz',
        FileDescription: 'CannonCommander',
        LegalCopyright: '',
        ProductName: 'CannonCommander',
        Comments: `A two-pane file manager. Build ${stamp}.`,
        InternalName: stamp,
      },
    },
  }
  writeFileSync(join(desktop, 'build', 'windows', 'info.json'), JSON.stringify(info, null, '\t') + '\n')
}

console.log(`building CannonCommander ${stamp}`)

// One fixed string through a shell: Node refuses to spawn npm's .cmd without a
// shell, and deprecates an argument array with one (DEP0190).
const ui = spawnSync('npm run build', { cwd: join(root, 'frontend'), stdio: 'inherit', shell: true })
if (ui.status !== 0) {
  console.error('the interface did not build, so the app would embed a stale copy')
  process.exit(ui.status ?? 1)
}
// The MuPDF headers the preview package compiles against.
run('go', ['run', './scripts/setup-gofitz-cgo'], { cwd: join(root, 'backend') })

rmSync(bin, { recursive: true, force: true })
rmSync(join(desktop, 'build', 'installer'), { recursive: true, force: true })
mkdirSync(bin, { recursive: true })

run('wails3', [
  'update', 'build-assets', '-silent',
  '-name', 'CannonCommander', '-binaryname', 'CannonCommander',
  '-config', 'build/config.yml', '-dir', 'build',
  '-productversion', plainVersion,
])
run('wails3', [
  'generate', 'icons', '-input', 'build/appicon.png',
  '-windowsfilename', 'build/windows/icon.ico', '-macfilename', 'build/darwin/icons.icns',
])

if (process.platform === 'win32') {
  writeWindowsVersion()
  const syso = join(desktop, `wails_windows_${arch}.syso`)
  run('wails3', [
    'generate', 'syso', '-arch', arch, '-icon', 'build/windows/icon.ico',
    '-manifest', 'build/windows/wails.exe.manifest', '-info', 'build/windows/info.json', '-out', syso,
  ])
  const exe = join(bin, 'CannonCommander.exe')
  try {
    goBuild(exe, { goos: 'windows', goarch: arch, tags: 'production', cgo: '0', ldflags: '-H windowsgui' })
  } finally {
    // Go links every .syso in the package, and one left behind would end up in
    // the next build for the other architecture.
    rmSync(syso, { force: true })
  }

  if (installer) {
    const installerSyso = join(desktop, 'installer', `installer_windows_${arch}.syso`)
    run('wails3', [
      'generate', 'syso', '-arch', arch, '-icon', 'build/windows/icon.ico',
      '-manifest', 'installer/installer.manifest', '-info', 'build/windows/info.json', '-out', installerSyso,
    ])
    try {
      buildInstaller(exe, `CannonCommander-${arch}-installer.exe`, { goos: 'windows', cgo: '0', tags: '', ldflags: '-H windowsgui' })
    } finally {
      rmSync(installerSyso, { force: true })
    }
  }
} else if (process.platform === 'darwin') {
  // Both architectures in one binary, so one download serves every Mac. The
  // deployment target matches what Wails' own template builds for.
  const mac = { MACOSX_DEPLOYMENT_TARGET: '12.0', CGO_CFLAGS: '-mmacosx-version-min=12.0', CGO_LDFLAGS: '-mmacosx-version-min=12.0' }
  for (const a of ['amd64', 'arm64']) {
    goBuild(join(bin, `CannonCommander-${a}`), { goos: 'darwin', goarch: a, tags: 'production,mupdf', env: mac })
  }
  const app = join(bin, 'CannonCommander.app', 'Contents')
  mkdirSync(join(app, 'MacOS'), { recursive: true })
  mkdirSync(join(app, 'Resources'), { recursive: true })
  run('lipo', ['-create', '-output', join(app, 'MacOS', 'CannonCommander'), join(bin, 'CannonCommander-amd64'), join(bin, 'CannonCommander-arm64')])
  rmSync(join(bin, 'CannonCommander-amd64'))
  rmSync(join(bin, 'CannonCommander-arm64'))
  cpSync(join(desktop, 'build', 'darwin', 'icons.icns'), join(app, 'Resources', 'icons.icns'))
  cpSync(join(desktop, 'build', 'darwin', 'Info.plist'), join(app, 'Info.plist'))
  // The helper programs, ffmpeg, 7-Zip and mutool, for each processor. The
  // bundle carries them, so an update of the bundle brings new ones along;
  // the app takes the set of its own processor (desktop/modules).
  mkdirSync(join(app, 'Resources', 'modules'), { recursive: true })
  for (const a of ['amd64', 'arm64']) {
    const dir = join(app, 'Resources', 'modules', a)
    run('go', ['run', './modules/assemble', '-platform', `darwin/${a}`, '-release', stamp, '-dir', dir])
    // --deep leaves programs under Resources alone, and Apple Silicon starts
    // none without a signature.
    for (const entry of readdirSync(dir, { recursive: true, withFileTypes: true })) {
      const file = join(entry.parentPath, entry.name)
      if (entry.isFile() && statSync(file).mode & 0o111) run('codesign', ['--force', '--sign', '-', file])
    }
  }
  // An ad-hoc signature over the whole bundle, as Wails' own template does:
  // Apple Silicon refuses to start code that carries none.
  run('codesign', ['--force', '--deep', '--sign', '-', join(bin, 'CannonCommander.app')])
} else {
  // GTK 3 and WebKitGTK 4.1, which every current distribution ships.
  goBuild(join(bin, 'CannonCommander'), { goos: 'linux', goarch: arch, tags: 'production,mupdf,gtk3' })
  if (installer) {
    buildInstaller(join(bin, 'CannonCommander'), `CannonCommander-${arch}-installer`, { goos: 'linux', cgo: '1', tags: 'gtk3' })
  }
}

console.log(`done: ${stamp}`)
