# wend

*[Русская версия](README.md)*

An interactive disk usage analyzer for the terminal.

It scans a directory in the background and shows the tree with sizes and
shares as it goes, while the right pane draws the same data as a sunburst
or a treemap — no desktop environment, right in your terminal.

```
 /home/user/projects  ·  19.3 GiB  ·  8k элементов
╭──────────────────────────────────────────────╮╭──────────────────────────────────────────────╮
│        доля объект                     размер││projects · 10 в этой папке · 8k всего · [спис…│
│█████   100% ▾ projects               19.3 GiB││  12.4 GiB   64%    4.2k  node_modules/       │
│███▎╌  64.3%   ▸ node_modules         12.4 GiB││   3.5 GiB   18%    1.4k  .git/               │
│▉╌╌╌╌  18.0%   ▸ .git                  3.5 GiB││   1.7 GiB    9%     180  assets/             │
│▍╌╌╌╌   8.7%   ▸ assets                1.7 GiB││   1.3 GiB    6%     320  build/              │
│▍╌╌╌╌   6.5%   ▸ build                 1.3 GiB││ 337.7 MiB    2%     910  vendor/             │
│▏╌╌╌╌   1.7%   ▸ vendor              337.7 MiB││ 118.8 MiB    1%     640  src/                │
│╌╌╌╌╌   0.6%   ▸ src                 118.8 MiB││  24.1 MiB    0%     260  tests/              │
│╌╌╌╌╌   0.1%   ▸ tests                24.1 MiB││  15.2 MiB    0%      74  docs/               │
│╌╌╌╌╌   0.1%   ▸ docs                 15.2 MiB││   1.8 MiB    0%       —  package-lock.json   │
│╌╌╌╌╌   0.0%     package-lock.json     1.8 MiB││  14.2 KiB    0%       —  README.md           │
│╌╌╌╌╌   0.0%     README.md            14.2 KiB││                                              │
╰──────────────────────────────────────────────╯╰──────────────────────────────────────────────╯
 /home/user/projects  ·  19.3 GiB  ·  8k элем.  ·  inode 4718952  ·  ссылок 12  ·  2026-07-26 …
 8k файлов / 8 папок  ·  j/k навигация  h/l свернуть/развернуть  v вид  tab панель  r перескан…
```

*(preview shown without color; charts and bars are colored in a real terminal)*

> **Note:** the application interface is currently in Russian only. The key
> bindings below are the same regardless, and this page explains what every
> column and label means. Interface localization is not implemented yet.

## Features

- **Incremental scan.** The tree fills in as the walk proceeds — no need to
  wait for it to finish before you start looking.
- **Four view modes:** list, horizontal bars, sunburst rings and treemap.
  Charts are drawn with colored character cells and account for the fact
  that a terminal cell is about twice as tall as it is wide.
- **Linked panes.** The branch under the cursor in the table is highlighted
  in the chart; everything else is dimmed.
- **Real filesystem data:** share of parent, recursive item count,
  modification time, inode, hard link count and actual on-disk usage.
- **Hard link deduplication** by inode — a file reachable through several
  links is counted once.
- **Responsive layout.** Columns drop out as the terminal narrows; nothing
  wraps and nothing breaks the layout.

## Installation

The binary is static and pulls in nothing at all — no libc, no runtime. It
runs on a musl-based distribution and inside a `FROM scratch` container
alike. Dropping a single file into your `PATH` is enough.

### One-liner

```sh
curl -fsSL https://raw.githubusercontent.com/Tinddd28/wend/main/scripts/install.sh | sh
```

The script detects your OS and architecture, downloads the latest release,
**verifies the checksum** and installs the binary into `/usr/local/bin`
(escalating with `sudo` if needed).

Useful variables:

```sh
PREFIX=$HOME/.local/bin sh install.sh   # install without sudo
VERSION=nightly         sh install.sh   # fresh build from main
VERSION=v1.0.0          sh install.sh   # a specific version
sh install.sh --dry-run                 # show what would be downloaded
```

If you would rather not pipe a downloaded script straight into a shell — and
that is a reasonable stance — fetch it, read it and then run it:

```sh
curl -fsSLO https://raw.githubusercontent.com/Tinddd28/wend/main/scripts/install.sh
less install.sh && sh install.sh
```

### Download a ready-to-run binary

Next to the archives on the [Releases](https://github.com/Tinddd28/wend/releases)
page there are **uncompressed binaries** you can download and run right away.
They are named `wend_<os>_<arch>`, deliberately without a version, so that
the permanent "latest" URL keeps working across releases:

```sh
curl -fsSLO https://github.com/Tinddd28/wend/releases/latest/download/wend_linux_amd64
chmod +x wend_linux_amd64
./wend_linux_amd64 ~/Downloads
```

### Download an archive

The archive is about three times smaller and ships `LICENSE` and `README`
alongside the binary:

```sh
tar -xzf wend_v1.0.0_linux_amd64.tar.gz
sha256sum -c SHA256SUMS --ignore-missing   # SHA256SUMS sits next to the archives
sudo install -m 0755 wend /usr/local/bin/wend
```

Windows archives are `.zip`, with `wend.exe` inside.

### Builds without waiting for a release

- **`nightly`** — a fresh build of every commit on `main`, available at
  permanent URLs such as
  `https://github.com/Tinddd28/wend/releases/download/nightly/wend_linux_amd64`.
  The tag moves with every commit: it is a "latest build" channel, not a
  version history.
- **CI artifacts** — every run on the
  [Actions](https://github.com/Tinddd28/wend/actions) page carries binaries
  for all six platforms at the bottom, pull requests included. They are kept
  for 30 days and require a GitHub login.

### go install

```sh
go install github.com/Tinddd28/wend/cmd/wend@latest
```

The binary lands in `$(go env GOPATH)/bin` — make sure that directory is on
your `PATH`.

### From source

Requires Go 1.26.5 or newer (the version is pinned in `go.mod`).

```sh
git clone https://github.com/Tinddd28/wend.git
cd wend
go build -o wend ./cmd/wend
```

## Usage

```sh
wend               # current directory
wend ~/Downloads   # a specific path
wend --version
wend --help
```

Directories that cannot be read due to permissions are skipped; the rest of
the walk continues uninterrupted.

## Key bindings

| Keys | Action |
| --- | --- |
| `j` / `k`, `↓` / `↑` | move up and down |
| `g` / `G`, `Home` / `End` | jump to top / bottom |
| `Ctrl+f` / `Ctrl+b`, `PgDn` / `PgUp` | page down / up |
| `l`, `→`, `Enter` | expand a directory, step into it |
| `h`, `←` | collapse a directory, go up to the parent |
| `Tab` | switch pane |
| `v` | cycle the main pane view mode |
| `r` | rescan from scratch |
| `q`, `Ctrl+c` | quit |

## View modes

Cycled with `v`:

1. **List** (`список`) — size, share of the parent's total, recursive item
   count, name.
2. **Bars** (`полосы`) — the same plus a horizontal bar scaled to the
   largest sibling.
3. **Rings** (`кольца`) — a sunburst: the hub shows the total, each
   successive ring is one nesting level, and a segment's angle is
   proportional to its size. Up to five rings; hue is derived from the
   segment's angular position, so nested entries keep their parent's color.
4. **Treemap** — a squarified layout: tile area is proportional to size,
   with up to three nesting levels and labels where they fit.

## Reading the numbers

A few places where `wend` deliberately departs from a naive count:

- **Share** is computed against the **parent's** total rather than the whole
  tree, which keeps it meaningful at any depth.
- **Hard links.** A file with `nlink > 1` is counted once, on the first link
  encountered. Repeats are marked `⇥`, shown with a zero contribution, and
  their total is reported in the status bar. This is why the grand total can
  be smaller than the sum of all file sizes.
- **On-disk size** (`blocks × 512`) appears in the detail line whenever it
  differs from the logical size by more than 10% — which happens with sparse
  files and because of rounding up to the block size. On platforms without
  `syscall.Stat_t` (Windows) inode, link count and on-disk size are
  unavailable, and deduplication does not run.
- **Symlinks** are not followed, so a symlink loop cannot send the scan into
  an infinite descent.

## Development

```sh
go test ./...          # tests
go test -race ./...    # with the race detector: the tree is read from the UI
                       # and written from the scanner goroutine
go vet ./...
gofmt -l ./cmd ./internal
```

Layout:

```
cmd/wend/              entry point, argument parsing
internal/scan/         filesystem walk and thread-safe result tree
internal/humanize/     formatting for sizes, counts and dates
internal/tui/          bubbletea model, layout, key handling
  ├── sidebar/         the tree table
  ├── mainview/        main pane and its view modes
  ├── statusbar/       details of the selected entry, plus hints
  ├── canvas/          raster canvas of character cells
  ├── chart/           sunburst and treemap rendering
  └── style/           palette and width-aware string clipping
```

## Building and publishing

Three workflows, all built on official actions and the `gh` CLI, with no
third-party dependencies:

| Workflow | Trigger | What it does |
| --- | --- | --- |
| [`ci.yml`](.github/workflows/ci.yml) | push to `main`, pull requests | gofmt, `go vet`, `go test -race`, builds for 6 platforms uploaded as artifacts |
| [`nightly.yml`](.github/workflows/nightly.yml) | push to `main` | tests, then refreshes the `nightly` prerelease — a fresh build at a permanent URL |
| [`release.yml`](.github/workflows/release.yml) | `v*` tag | checks, tests, archives with `SHA256SUMS`, a GitHub Release with generated notes |

Cutting a release:

```sh
git tag -a v1.0.0 -m "v1.0.0"
git push origin v1.0.0
```

Tags containing a hyphen (`v1.0.0-rc1`) are marked as prereleases and do not
take over as "latest".

The same artifacts can be produced locally, without waiting for CI:

```sh
./scripts/build-release.sh v1.0.0   # output in dist/
```

Every workflow takes its Go version from `go.mod` so that CI cannot drift
away from the project. Builds always run with `CGO_ENABLED=0` and
`-trimpath` — hence the static, reproducible binary.

## License

[MIT](LICENSE)
