// The languages the editor colours. Each loads as a chunk of its own the first time a file needs it: CodeMirror's own
// packages where there is one, a mode from CodeMirror 5 for the rest. A file gets its language from its name, else
// from a shebang or the start of its first line.
import { type LanguageSupport, StreamLanguage, type StreamParser } from '@codemirror/language';
import type { Extension } from '@codemirror/state';

export interface Language {
  id: string;
  name: string;
  ext?: string[];
  // Whole names, matched without regard to case, such as Dockerfile.
  files?: string[];
  // The programs a shebang names, without their version, such as python for python3.11.
  runs?: string[];
  load: () => Promise<LanguageSupport | Extension>;
}

const legacy = (parser: () => Promise<StreamParser<unknown>>) => async () => StreamLanguage.define(await parser());

export const LANGUAGES: Language[] = [
  { id: 'c', name: 'C', ext: ['c', 'h'], load: async () => (await import('@codemirror/lang-cpp')).cpp() },
  { id: 'cpp', name: 'C++', ext: ['cpp', 'cc', 'cxx', 'c++', 'hpp', 'hh', 'hxx', 'ino'], load: async () => (await import('@codemirror/lang-cpp')).cpp() },
  { id: 'csharp', name: 'C#', ext: ['cs', 'csx'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/clike')).csharp) },
  { id: 'clojure', name: 'Clojure', ext: ['clj', 'cljs', 'cljc', 'edn'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/clojure')).clojure) },
  { id: 'cmake', name: 'CMake', ext: ['cmake'], files: ['CMakeLists.txt'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/cmake')).cmake) },
  { id: 'coffeescript', name: 'CoffeeScript', ext: ['coffee'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/coffeescript')).coffeeScript) },
  { id: 'lisp', name: 'Common Lisp', ext: ['lisp', 'lsp', 'cl', 'el'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/commonlisp')).commonLisp) },
  { id: 'crystal', name: 'Crystal', ext: ['cr'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/crystal')).crystal) },
  { id: 'css', name: 'CSS', ext: ['css'], load: async () => (await import('@codemirror/lang-css')).css() },
  { id: 'd', name: 'D', ext: ['d', 'di'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/d')).d) },
  { id: 'dart', name: 'Dart', ext: ['dart'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/clike')).dart) },
  { id: 'diff', name: 'Diff', ext: ['diff', 'patch', 'rej'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/diff')).diff) },
  { id: 'dockerfile', name: 'Dockerfile', ext: ['dockerfile', 'containerfile'], files: ['Dockerfile', 'Containerfile'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/dockerfile')).dockerFile) },
  { id: 'elm', name: 'Elm', ext: ['elm'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/elm')).elm) },
  { id: 'erlang', name: 'Erlang', ext: ['erl', 'hrl'], runs: ['escript'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/erlang')).erlang) },
  { id: 'fortran', name: 'Fortran', ext: ['f', 'for', 'f90', 'f95', 'f03', 'f08'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/fortran')).fortran) },
  { id: 'fsharp', name: 'F#', ext: ['fs', 'fsi', 'fsx'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/mllike')).fSharp) },
  { id: 'gherkin', name: 'Gherkin', ext: ['feature'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/gherkin')).gherkin) },
  { id: 'go', name: 'Go', ext: ['go'], load: async () => (await import('@codemirror/lang-go')).go() },
  { id: 'groovy', name: 'Groovy', ext: ['groovy', 'gradle', 'gvy'], files: ['Jenkinsfile'], runs: ['groovy'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/groovy')).groovy) },
  { id: 'haskell', name: 'Haskell', ext: ['hs'], runs: ['runhaskell'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/haskell')).haskell) },
  { id: 'html', name: 'HTML', ext: ['html', 'htm', 'xhtml', 'shtml'], load: async () => (await import('@codemirror/lang-html')).html() },
  { id: 'http', name: 'HTTP', ext: ['http'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/http')).http) },
  { id: 'ini', name: 'INI', ext: ['ini', 'cfg', 'properties', 'env', 'editorconfig', 'gitconfig', 'npmrc', 'desktop', 'inf', 'reg', 'service', 'timer', 'socket', 'mount', 'network'],
    files: ['.gitconfig', '.npmrc', '.editorconfig', 'ident'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/properties')).properties) },
  { id: 'java', name: 'Java', ext: ['java'], load: async () => (await import('@codemirror/lang-java')).java() },
  { id: 'javascript', name: 'JavaScript', ext: ['js', 'mjs', 'cjs'], runs: ['node', 'nodejs', 'deno', 'bun'], load: async () => (await import('@codemirror/lang-javascript')).javascript() },
  { id: 'jinja', name: 'Jinja', ext: ['j2', 'jinja', 'jinja2'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/jinja2')).jinja2) },
  { id: 'json', name: 'JSON', ext: ['json', 'jsonc', 'json5', 'webmanifest', 'geojson', 'har'], files: ['.babelrc', '.eslintrc', '.prettierrc'], load: async () => (await import('@codemirror/lang-json')).json() },
  { id: 'jsx', name: 'JSX', ext: ['jsx'], load: async () => (await import('@codemirror/lang-javascript')).javascript({ jsx: true }) },
  { id: 'julia', name: 'Julia', ext: ['jl'], runs: ['julia'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/julia')).julia) },
  { id: 'kotlin', name: 'Kotlin', ext: ['kt', 'kts'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/clike')).kotlin) },
  { id: 'latex', name: 'LaTeX', ext: ['tex', 'sty', 'cls', 'ltx', 'bib'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/stex')).stex) },
  { id: 'less', name: 'Less', ext: ['less'], load: async () => (await import('@codemirror/lang-less')).less() },
  { id: 'lua', name: 'Lua', ext: ['lua'], runs: ['lua', 'luajit'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/lua')).lua) },
  // CodeMirror has no mode for makefiles. The shell's is close: comments, strings and variables.
  { id: 'makefile', name: 'Makefile', ext: ['mk', 'mak'], files: ['Makefile', 'GNUmakefile', 'makefile'], runs: ['make'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/shell')).shell) },
  { id: 'markdown', name: 'Markdown', ext: ['md', 'markdown', 'mkd', 'mdown'], load: async () => (await import('@codemirror/lang-markdown')).markdown() },
  { id: 'matlab', name: 'MATLAB', ext: ['m'], runs: ['octave'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/octave')).octave) },
  { id: 'nginx', name: 'nginx', ext: ['conf', 'nginx'], files: ['nginx.conf'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/nginx')).nginx) },
  { id: 'nsis', name: 'NSIS', ext: ['nsi', 'nsh'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/nsis')).nsis) },
  { id: 'ocaml', name: 'OCaml', ext: ['ml', 'mli'], runs: ['ocaml'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/mllike')).oCaml) },
  { id: 'pascal', name: 'Pascal', ext: ['pas', 'dpr', 'lpr'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/pascal')).pascal) },
  { id: 'perl', name: 'Perl', ext: ['pl', 'pm', 'perl'], runs: ['perl'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/perl')).perl) },
  { id: 'php', name: 'PHP', ext: ['php', 'phtml', 'php3', 'php4', 'php5', 'phps'], runs: ['php'], load: async () => (await import('@codemirror/lang-php')).php() },
  { id: 'powershell', name: 'PowerShell', ext: ['ps1', 'psm1', 'psd1'], runs: ['pwsh', 'powershell'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/powershell')).powerShell) },
  { id: 'protobuf', name: 'Protocol Buffers', ext: ['proto'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/protobuf')).protobuf) },
  { id: 'pug', name: 'Pug', ext: ['pug', 'jade'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/pug')).pug) },
  { id: 'puppet', name: 'Puppet', ext: ['pp'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/puppet')).puppet) },
  { id: 'python', name: 'Python', ext: ['py', 'pyw', 'pyi', 'pyx', 'gyp'], files: ['SConstruct', 'SConscript'], runs: ['python', 'pypy'], load: async () => (await import('@codemirror/lang-python')).python() },
  { id: 'r', name: 'R', ext: ['r', 'rprofile'], runs: ['rscript'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/r')).r) },
  { id: 'ruby', name: 'Ruby', ext: ['rb', 'rake', 'gemspec', 'ru'], files: ['Gemfile', 'Rakefile', 'Vagrantfile', 'Podfile', 'Brewfile'], runs: ['ruby'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/ruby')).ruby) },
  { id: 'rust', name: 'Rust', ext: ['rs'], load: async () => (await import('@codemirror/lang-rust')).rust() },
  { id: 'sass', name: 'Sass', ext: ['sass'], load: async () => (await import('@codemirror/lang-sass')).sass({ indented: true }) },
  { id: 'scala', name: 'Scala', ext: ['scala', 'sbt'], runs: ['scala'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/clike')).scala) },
  { id: 'scheme', name: 'Scheme', ext: ['scm', 'ss', 'rkt'], runs: ['guile', 'racket'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/scheme')).scheme) },
  { id: 'scss', name: 'SCSS', ext: ['scss'], load: async () => (await import('@codemirror/lang-sass')).sass() },
  // Unraid starts its array with a script called go, without an extension.
  { id: 'shell', name: 'Shell', ext: ['sh', 'bash', 'zsh', 'ksh', 'ash', 'bashrc', 'zshrc', 'profile', 'command'],
    files: ['go', '.bashrc', '.bash_profile', '.bash_aliases', '.profile', '.zshrc', '.zprofile', 'PKGBUILD', 'APKBUILD'],
    runs: ['sh', 'bash', 'zsh', 'ksh', 'dash', 'ash', 'busybox'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/shell')).shell) },
  { id: 'smalltalk', name: 'Smalltalk', ext: ['st'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/smalltalk')).smalltalk) },
  { id: 'sparql', name: 'SPARQL', ext: ['rq', 'sparql'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/sparql')).sparql) },
  { id: 'sql', name: 'SQL', ext: ['sql', 'ddl', 'dml'], load: async () => (await import('@codemirror/lang-sql')).sql() },
  { id: 'rpmspec', name: 'RPM Spec', ext: ['spec'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/rpm')).rpmSpec) },
  { id: 'swift', name: 'Swift', ext: ['swift'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/swift')).swift) },
  { id: 'tcl', name: 'Tcl', ext: ['tcl', 'tk'], runs: ['tclsh', 'wish'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/tcl')).tcl) },
  { id: 'toml', name: 'TOML', ext: ['toml'], files: ['Cargo.lock', 'Pipfile', 'poetry.lock'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/toml')).toml) },
  { id: 'troff', name: 'troff', ext: ['man', 'roff', 'mdoc'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/troff')).troff) },
  { id: 'tsx', name: 'TSX', ext: ['tsx'], load: async () => (await import('@codemirror/lang-javascript')).javascript({ typescript: true, jsx: true }) },
  { id: 'turtle', name: 'Turtle', ext: ['ttl'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/turtle')).turtle) },
  { id: 'typescript', name: 'TypeScript', ext: ['ts', 'mts', 'cts'], runs: ['ts-node', 'tsx'], load: async () => (await import('@codemirror/lang-javascript')).javascript({ typescript: true }) },
  { id: 'vb', name: 'Visual Basic', ext: ['vb', 'bas'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/vb')).vb) },
  { id: 'vbscript', name: 'VBScript', ext: ['vbs', 'vbe'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/vbscript')).vbScript) },
  { id: 'verilog', name: 'Verilog', ext: ['v', 'sv', 'svh', 'vh'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/verilog')).verilog) },
  { id: 'vhdl', name: 'VHDL', ext: ['vhd', 'vhdl'], load: legacy(async () => (await import('@codemirror/legacy-modes/mode/vhdl')).vhdl) },
  { id: 'vue', name: 'Vue', ext: ['vue'], load: async () => (await import('@codemirror/lang-vue')).vue() },
  { id: 'xml', name: 'XML', ext: ['xml', 'svg', 'plist', 'xsd', 'xsl', 'xslt', 'rss', 'atom', 'csproj', 'vbproj', 'props', 'targets', 'resx', 'kml', 'gpx', 'opf', 'nfo', 'xaml', 'wsdl'],
    load: async () => (await import('@codemirror/lang-xml')).xml() },
  { id: 'yaml', name: 'YAML', ext: ['yml', 'yaml'], files: ['.clang-format'], load: async () => (await import('@codemirror/lang-yaml')).yaml() },
];

export const byId = (id: string) => LANGUAGES.find((l) => l.id === id);

const byExt = new Map(LANGUAGES.flatMap((l) => (l.ext ?? []).map((e) => [e, l] as const)));
const byFile = new Map(LANGUAGES.flatMap((l) => (l.files ?? []).map((f) => [f.toLowerCase(), l] as const)));
const byProgram = new Map(LANGUAGES.flatMap((l) => (l.runs ?? []).map((p) => [p, l] as const)));

// The program a shebang runs, through env as well, without its version: python for #!/usr/bin/env -S python3.11 -u.
export function shebang(first: string): string {
  const m = first.match(/^#!\s*(\S+)(.*)$/);
  if (!m) return '';
  let prog = m[1]!.split('/').pop()!;
  if (prog === 'env') prog = m[2]!.trim().split(/\s+/).find((w) => !w.startsWith('-') && !w.includes('=')) ?? '';
  return prog.toLowerCase().replace(/[\d.]+$/, '');
}

export function detect(name: string, first = ''): Language | undefined {
  const lower = name.toLowerCase();
  const own = byFile.get(lower);
  if (own) return own;
  if (/^(docker|container)file\./.test(lower)) return byId('dockerfile');
  const dot = lower.lastIndexOf('.');
  const ext = dot >= 0 ? lower.slice(dot + 1) : '';
  // A file such as notes.txt or app.log says it is plain text even when it starts like a script.
  if (ext === 'txt' || ext === 'log') return undefined;
  const found = ext && byExt.get(ext);
  if (found) return found;
  const line = first.trimStart().split('\n', 1)[0]!.slice(0, 200).toLowerCase();
  const prog = shebang(line);
  if (prog) return byProgram.get(prog);
  if (line.startsWith('<?php')) return byId('php');
  if (line.startsWith('<!doctype html') || line.startsWith('<html')) return byId('html');
  if (line.startsWith('<?xml')) return byId('xml');
  return undefined;
}
