---
title: GitHub Release Automation & Linux Binary Distribution
status: in_progress
started: 2026-11-15
completed:
branch: dev
priority: medium
---

# GitHub Release Automation & Linux Binary Distribution

## Objective
Rilis tui-workflow lewat git tag `vX.Y.Z`: GitHub Actions otomatis build binary Linux (amd64 + arm64) dan publish GitHub Release dengan asset + checksum, pengguna install satu baris lewat `install.sh` dari README, dan versi aplikasi punya satu sumber kebenaran (`internal/buildinfo.Version`, di-inject via ldflags dari tag).

**Goal:** Tag di-push → binary Linux (amd64/arm64) + GitHub Release + install script siap dipakai.
**Architecture:** Tag `v*` memicu workflow release (test → matrix build → sha256 → `gh release create --generate-notes`). `internal/buildinfo.Version` adalah default dev; ldflags `-X` menimpanya dari tag di Makefile, CI, dan release workflow. `scripts/release.sh` (dijalankan user manual) me-update versi, commit, tag, dan push sekaligus.
**Tech Stack:** Go 1.24+ (repo `go.mod`: go 1.26.7), GitHub Actions (ubuntu-latest, go-version-file), `gh` CLI (preinstalled di runner), bash, wget.
**Spec:** Design singkat yang disetujui user (sesi 2026-11-15) — tercakup penuh di plan ini; tidak ada design doc terpisah. Lokasi plan mengikuti `AGENTS.md` repo (`task/CURRENT.md`).

## Global Constraints
- Module path `tui-workflow` — target ldflags wajib `-X tui-workflow/internal/buildinfo.Version=<ver>`.
- Nama binary release: **`workflow`** (keputusan user — bukan `tui-workflow`); asset `workflow_<ver>_linux_<arch>.tar.gz` berisi binary `workflow`.
- Tag format wajib `vX.Y.Z` (`v` + semver tiga komponen).
- Target platform **Linux only** (amd64 + arm64 via cross-compile). macOS/Windows di luar scope.
- Release notes: `gh release create --generate-notes`; tidak ada `CHANGELOG.md`.
- `go vet ./...` wajib bersih; `go test -race ./...` wajib hijau; `wc -l internal/app/*.go` ≤ 500 (Rule 12).
- DILARANG agent menjalankan `git add/commit/tag/push` — semua operasi git menulis dilakukan user lewat `scripts/release.sh`.
- Versi default di kode (`0.2.0`) dan PRD/About yang sudah ada saat ini adalah `v0.2.0` — jangan turunkan.

## Review Focus
Lima kegagalan yang paling mungkin menggigit, beserta test yang menguncinya:
1. **`install.sh` salah deteksi arsitektur** (mis. `arm64` vs `aarch64`, atau mesin RISC/ppc) → script wajib exit dengan pesan error jelas, tidak mengunduh apa pun. Test: jalankan dengan arch palsu (`uname` stub) → expect exit ≠ 0 + pesan "unsupported architecture". Diuji manual di checklist (bukan unit test — bash).
2. **Download parsial/corrupt diinstall** → wajib verifikasi SHA-256 dari `checksums.txt` sebelum `install`; gagal → exit 1, binary tidak tersentuh. Test: ubah checksum lokal → expect abort tanpa menulis ke `~/.local/bin`.
3. **ldflags gagal menimpa versi** (mis. path package salah) → binary release menampilkan default `0.2.0`. Test: `go build -ldflags "-X tui-workflow/internal/buildinfo.Version=9.9.9-test" -o /tmp/workflow-test ./cmd/tui-workflow && /tmp/workflow-test --version` → expect `v9.9.9-test`. (Flag `--version` ditambah di Task 1.)
4. **`scripts/release.sh` jalan di kondisi buruk** (dirty tree, bukan branch main, tag sudah ada, baris revision PRD belum ditambah) → preflight wajib abort dengan pesan, tidak ada commit/tag/push parsial. Test: masing-masing kondisi dipicu manual sekali sebelum rilis pertama (checklist).
5. **Workflow release gagal di tengah (test/build/release)** → job `release` butuh `permissions: contents: write`; kegagalan diketahui dari status Actions. Rollback rilis: hapus tag remote + draft release + re-tag setelah fix (didokumentasikan di `docs/release.md`).

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| About modal (`view_modals.go`) | String jadi `fmt.Sprintf`; tinggi baris tetap, 1 baris judul tetap | Verifikasi About modal visual + `go test ./internal/app/...` |
| Top Menu / Tab Bar / Status Bar | Tidak tersentuh | — |

## UI / Keymap Flow
- Tidak ada keymap baru. `workflow --version` (flag CLI baru) mencetak `tui-workflow v<ver>` lalu exit 0 — sebelum TUI start; berguna untuk verifikasi install dari README.
- About modal menampilkan versi dari `buildinfo.Version` (sebelumnya hardcode `v0.2.0`).

## Ground Truth Findings
- Repo `github.com/ajikamaludin-dev/tui-workflow`, branch lokal hanya `dev`; **belum ada branch `main`**, belum ada `.github/workflows/`, belum ada tag (`git tag -l` kosong).
- Versi di-hardcode di `internal/app/view_modals.go:115` (`tui-workflow v0.2.0` dalam raw string `content :=`); `PRD.md:3` (`**Version:** v0.2.0`, + revision history table); `README.md:7` (badge `PRD-v0.2.0`).
- `go.mod`: module `tui-workflow`, `go 1.26.7`. `cmd/tui-workflow/main.go` belum punya `--version`.
- `Makefile`: `BINARY ?= workflow`, `INSTALL_DIR ?= $(HOME)/.local/bin`, `LDFLAGS ?= -s -w`, build pakai `-trimpath`. README prose masih menyebut `tui-workflow` sebagai nama binary (inkonsisten — dirapikan di Task 6).
- PRD revision history format baris: `| v0.2.0 | 2026-11-14 | ... |` di `PRD.md` bagian `## Revision History`.
- Runner `ubuntu-latest` punya `gh` preinstalled; `GITHUB_TOKEN` otomatis tersedia bila `permissions: contents: write`.

## Plan — Files Affected & Execution Steps

### Task 1: `internal/buildinfo` + wire versi (About modal, `--version`, Makefile)

**Files:**
- Create: `internal/buildinfo/version.go`
- Create: `internal/buildinfo/version_test.go`
- Modify: `internal/app/view_modals.go` (renderAboutModal, ±baris 108–130)
- Modify: `cmd/tui-workflow/main.go` (flag `--version`)
- Modify: `Makefile` (`VERSION`, `LDFLAGS`)

**Interfaces:**
- Produces: `buildinfo.Version string` — dipakai Task 2/3 (ldflags), `main.go`, `view_modals.go`.

- [ ] **Step 1: Write the failing test** `internal/buildinfo/version_test.go`:

```go
package buildinfo

import "testing"

func TestVersionIsSemverTriple(t *testing.T) {
	v := Version
	if v == "" {
		t.Fatal("Version must not be empty")
	}
	for i, part := range strings.Split(v, ".") {
		if _, err := strconv.Atoi(part); err != nil {
			t.Fatalf("part %d (%q) not numeric: %v", i, part, err)
		}
	}
}
```

(import `strconv`, `strings`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/buildinfo/ -run TestVersion -v`
Expected: FAIL — package tidak ada.

- [ ] **Step 3: Create `internal/buildinfo/version.go`:**

```go
// Package buildinfo holds build-time metadata.
// Version is the default for local/dev builds; release builds override it
// with -ldflags "-X tui-workflow/internal/buildinfo.Version=x.y.z".
package buildinfo

// Version is the application version (semver without leading v).
var Version = "0.2.0"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/buildinfo/ -run TestVersion -v` → PASS.

- [ ] **Step 5: Wire into About modal** — di `internal/app/view_modals.go` renderAboutModal: ganti `content := \`tui-workflow v0.2.0` menjadi `content := fmt.Sprintf("tui-workflow v%s\\nModern, High-Performance Terminal Workflow Environment.\\n\\nCore Features ...", buildinfo.Version)`. Pertahankan seluruh isi body lainnya verbatim; pastikan `fmt` sudah di-import dan tambahkan import `tui-workflow/internal/buildinfo`. (`content` bukan lagi raw string; semua `\n` eksplisit.)

- [ ] **Step 6: Add `--version` flag ke `cmd/tui-workflow/main.go`** — di loop arg, sebelum `targetPath` logic: jika arg == `--version` / `-v`, print `tui-workflow v<buildinfo.Version>` ke stdout dan `os.Exit(0)`. Import `tui-workflow/internal/buildinfo`.

- [ ] **Step 7: Update `Makefile`** — tambah setelah `LDFLAGS ?= -s -w`:

```make
VERSION     ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')
ifeq ($(VERSION),)
VERSION     := 0.2.0
endif
LDFLAGS     += -X tui-workflow/internal/buildinfo.Version=$(VERSION)
```

(BINARY tetap `workflow`.) `make build` output tetap `bin/workflow`.

- [ ] **Step 8: Verify injection end-to-end**

Run: `go build -trimpath -ldflags "-s -w -X tui-workflow/internal/buildinfo.Version=9.9.9-test" -o /tmp/workflow-vercheck ./cmd/tui-workflow && /tmp/workflow-vercheck --version`
Expected: `tui-workflow v9.9.9-test`. Lalu `rm -f /tmp/workflow-vercheck`.

- [ ] **Step 9: Full check**

Run: `go vet ./... && go test -race ./... && wc -l internal/app/*.go | tail -1`
Expected: vet bersih, test hijau, `internal/app` files ≤ 500 baris.

### Task 2: CI workflow (`.github/workflows/ci.yml`)

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create workflow**

```yaml
name: CI
on:
  push:
    branches: [main, dev]
  pull_request:
    branches: [main]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go vet ./...
      - run: go test -race ./...
      - run: go build ./...
```

- [ ] **Step 2: Validate YAML** — `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml'))"` (atau `actionlint` bila terpasang). Expected: exit 0.

### Task 3: Release workflow (`.github/workflows/release.yml`)

**Files:**
- Create: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: tag `vX.Y.Z` di `GITHUB_REF_NAME`; `internal/buildinfo.Version` dari Task 1; binary & asset naming `workflow_<ver>_linux_<arch>.tar.gz` (Global Constraints).

- [ ] **Step 1: Create workflow**

```yaml
name: Release
on:
  push:
    tags: ['v*']
permissions:
  contents: write
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go vet ./...
      - run: go test -race ./...
  build:
    needs: test
    runs-on: ubuntu-latest
    strategy:
      matrix:
        arch: [amd64, arm64]
    env:
      GOOS: linux
      GOARCH: ${{ matrix.arch }}
      CGO_ENABLED: 0
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Build
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          go build -trimpath -ldflags "-s -w -X tui-workflow/internal/buildinfo.Version=${VERSION}" -o dist/workflow ./cmd/tui-workflow
      - name: Package & checksum
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          tar -czf "workflow_${VERSION}_linux_${GOARCH}.tar.gz" -C dist workflow
          sha256sum "workflow_${VERSION}_linux_${GOARCH}.tar.gz" > "checksums-linux-${GOARCH}.txt"
      - uses: actions/upload-artifact@v4
        with:
          name: workflow-linux-${{ matrix.arch }}
          path: |
            workflow_*_linux_${{ matrix.arch }}.tar.gz
            checksums-linux-${{ matrix.arch }}.txt
  release:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          pattern: workflow-linux-*
          merge-multiple: true
      - name: Merge checksums
        run: cat checksums-linux-*.txt > checksums.txt
      - name: Create GitHub Release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: gh release create "${GITHUB_REF_NAME}" --generate-notes --title "${GITHUB_REF_NAME}" workflow_*_linux_*.tar.gz checksums.txt
```

- [ ] **Step 2: Validate YAML** (sama seperti Task 2 Step 2).

- [ ] **Step 3: Dry-check asset naming** — nama `workflow_<ver>_linux_<arch>.tar.gz` harus persis sama dengan yang di-generate Task 4 (`install.sh`); cocokkan dua-duanya.

### Task 4: `install.sh` (root repo) + README one-liner

**Files:**
- Create: `install.sh`

**Interfaces:**
- Consumes: asset naming dari Task 3; repo `ajikamaludin-dev/tui-workflow`.

- [ ] **Step 1: Create `install.sh`** — bash, `set -euo pipefail`. Argumen opsional `$1` = versi tag (mis. `v0.3.0`; default: ambil dari `https://api.github.com/repos/ajikamaludin-dev/tui-workflow/releases/latest` via wget+grep `tag_name`). Alur wajib:

1. Deteksi arch: `uname -m` → `x86_64`=amd64, `aarch64`/`arm64`=arm64, else error + exit 1.
2. Hitung `VER="${TAG#v}"`, `URL="https://github.com/ajikamaludin-dev/tui-workflow/releases/download/${TAG}/workflow_${VER}_linux_${ARCH}.tar.gz"`.
3. Unduh tarball + `checksums.txt` ke `mktemp -d` (trap cleanup EXIT).
4. Verifikasi: `grep "workflow_${VER}_linux_${ARCH}.tar.gz" checksums.txt | sha256sum -c -` → gagal = exit 1.
5. `mkdir -p "$HOME/.local/bin"`, `tar -xzf` tarball ke temp, `install -m 0755 workflow "$HOME/.local/bin/workflow"`.
6. Cek `\$HOME/.local/bin` di `PATH`; bila tidak, print instruksi `export PATH="$HOME/.local/bin:$PATH"`.
7. Print `workflow installed: $($HOME/.local/bin/workflow --version || echo unknown)`.

Gunakan `wget -q` (fallback `curl -fsSL` bila wget tidak ada). `chmod 0755 install.sh`.

- [ ] **Step 2: Shell-check** — `bash -n install.sh`; bila `shellcheck` terpasang jalankan juga (0 error).

### Task 5: `scripts/release.sh` — rilis satu perintah

**Files:**
- Create: `scripts/release.sh`

- [ ] **Step 1: Create script** — `set -euo pipefail`, pemakaian: `./scripts/release.sh vX.Y.Z`. Wajib berurutan:

1. Validasi arg: regex `^v[0-9]+\.[0-9]+\.[0-9]+$` else usage + exit 1.
2. Preflight abort bila: branch bukan `main`; `git status --porcelain` tidak kosong; `git rev-parse "$TAG" >/dev/null 2>&1` (tag ada) atau `git ls-remote --tags origin "refs/tags/$TAG"` non-kosong; `grep -q "| ${TAG} |" PRD.md` gagal (revision history belum diisi) → pesan "tambahkan baris revision PRD untuk ${TAG} dulu".
3. `go vet ./...` lalu `go test -race ./...`.
4. `VER="${TAG#v}"`; `sed -i -E "s/^(var Version = )\"[^\"]+\"/\1\"${VER}\"/" internal/buildinfo/version.go`; `sed -i -E "s/^\*\*Version:\*\* v[0-9.]+/**Version:** ${TAG}/" PRD.md`.
5. `git add internal/buildinfo/version.go PRD.md && git commit -m "chore(release): ${TAG}"`.
6. `git tag -a "$TAG" -m "Release ${TAG}"`.
7. `git push origin main --follow-tags`.
8. Print: link release `https://github.com/ajikamaludin-dev/tui-workflow/releases/tag/${TAG}` + catatan "workflow Release sedang berjalan; pantau tab Actions".

`chmod 0755 scripts/release.sh`. Script ini sengaja berisi commit+tag+push dan **hanya** dijalankan user secara sadar — agent tidak pernah menjalankannya.

- [ ] **Step 2: Shell-check** — `bash -n scripts/release.sh` (+ `shellcheck` bila ada).

### Task 6: Dokumentasi — README, PRD, `docs/release.md`

**Files:**
- Modify: `README.md`
- Modify: `PRD.md` (hanya baris revision history — ditambahkan user saat rilis; plan ini menambah baris untuk rilis pertama bila versinya sudah ditentukan, atau placeholder dihapus)
- Create: `docs/release.md`

- [ ] **Step 1: README — section instalasi baru** sebelum bagian "Kompilasi Binary Mandiri":

```markdown
### Install Cepat (Linux)
```bash
wget -qO- https://raw.githubusercontent.com/ajikamaludin-dev/tui-workflow/main/install.sh | sh
```
Atau download manual dari [Releases](https://github.com/ajikamaludin-dev/tui-workflow/releases) → pilih asset `workflow_<ver>_linux_<amd64|arm64>.tar.gz` sesuai arsitektur (`uname -m`). Install ke `~/.local/bin/workflow`. Verifikasi: `workflow --version`.
```

- [ ] **Step 2: README — badge** ganti baris 7 (`PRD-v0.2.0`) dengan:

`[![Latest Release](https://img.shields.io/github/v/release/ajikamaludin-dev/tui-workflow)](https://github.com/ajikamaludin-dev/tui-workflow/releases)`

- [ ] **Step 3: README — samakan nama binary** di section build: `go build -o tui-workflow` → `-o workflow`; pastikan prose `~/.local/bin/workflow` konsisten.

- [ ] **Step 4: Create `docs/release.md`** — alur lengkap: (1) semua fitur di-merge ke `dev`; (2) merge `dev` → `main` (CI harus hijau); (3) tambahkan baris revision PRD (`| vX.Y.Z | YYYY-MM-DD | ... |`); (4) jalankan `./scripts/release.sh vX.Y.Z`; (5) workflow `Release` otomatis membuat GitHub Release + asset + `checksums.txt`; (6) verifikasi: `workflow --version` setelah install via `install.sh`, cek About modal; (7) rollback: `gh release delete vX.Y.Z --yes && git push --delete origin vX.Y.Z && git tag -d vX.Y.Z`, perbaiki, tag ulang. Catatan: merge ke `main` saja tidak membuat release — hanya tag.

### Task 7: Branch `main` + verifikasi end-to-end (manual, butuh approval user)

- [ ] **Step 1 (approval dulu):** user menyetujui pembuatan & push branch `main` (dari `dev`) + set default branch GitHub: `git branch main dev && git push -u origin main` lalu `gh repo edit ajikamaludin-dev/tui-workflow --default-branch main`. Agent TIDAK menjalankannya tanpa approval eksplisit.
- [ ] **Step 2:** user menjalankan verifikasi TestVersion di mesin bersih atau container: `wget -qO- .../install.sh | sh && workflow --version`.
- [ ] **Step 3:** rilis pertama nyata: user menambah baris revision PRD untuk `v0.3.0` (usulan), lalu `./scripts/release.sh v0.3.0`, pantau Actions, cek Release page + asset + checksum cocok dengan yang diunduh install.sh.

## Tasks (ringkas)
- [x] Task 1: buildinfo + wire versi + `--version` + Makefile
- [x] Task 2: ci.yml
- [x] Task 3: release.yml
- [x] Task 4: install.sh
- [x] Task 5: scripts/release.sh
- [x] Task 6: README + PRD + docs/release.md
- [ ] Task 7: branch `main` + release pertama (butuh approval)
- [ ] Verification: go vet ./... ✓, go test -v -race ./... ✓, wc -l internal/app/*.go ≤ 500 ✓

## Manual Test Checklist
- [ ] `go build ./... && go run ./cmd/tui-workflow --version` → mencetak `tui-workflow v0.3.0` (atau 9.9.9-test untuk cek injeksi)
- [ ] `make build` → `bin/workflow`; `make install` → `~/.local/bin/workflow`; `workflow --version` jalan
- [ ] Buka About modal (Alt+A) → versi tampil dari buildinfo, bukan hardcode
- [ ] `bash -n install.sh && bash -n scripts/release.sh` → exit 0
- [ ] `install.sh` di shell dengan `uname -m` dipalsukan ke `riscv64` → error jelas, exit 1, tidak ada unduhan
- [ ] `checksums.txt` rusak (1 byte) → install.sh abort, `~/.local/bin/workflow` tidak tersentuh
- [ ] `scripts/release.sh v1.2.3` dijalankan di branch `dev` → abort dengan pesan "harus di branch main"
- [ ] `scripts/release.sh` dengan working tree kotor → abort
- [ ] Setelah rilis: `wget -qO- https://raw.githubusercontent.com/ajikamaludin-dev/tui-workflow/main/install.sh | sh` di mesin/user bersih → `workflow --version` = versi rilis terbaru; checksum OK
- [ ] Halaman Releases menampilkan tarball amd64 + arm64 + `checksums.txt`

## Acceptance Criteria
- [ ] `git tag -l` di main menampilkan tag rilis; Release GitHub berisi 2 tarball + checksums; notes ter-generate otomatis
- [ ] `workflow --version` menampilkan versi dari ldflags (bukan default) di binary hasil release
- [ ] About modal menampilkan versi buildinfo
- [ ] README menampilkan instruksi install one-liner yang berfungsi
- [ ] `go vet ./...` bersih, `go test -race ./...` hijau
- [ ] `docs/release.md` mendokumentasikan alur rilis + rollback
- [ ] Semua langkah yang menulis git (branch main, push, tag) dieksekusi hanya setelah approval eksplisit user

## Decisions
- Opsi B (workflow polos + `gh release create`), bukan GoReleaser — dipilih user; lebih transparan, minim dependensi.
- Tag manual (bukan release-please/auto-bump dari merge) — dipilih user; versi diputuskan manusia.
- Binary bernama `workflow` (bukan `tui-workflow`) — keputusan eksplisit user 2026-11-15; README diselaraskan.
- Build target Linux only (amd64+arm64) — permintaan user; cross-compile, tidak butuh mesin ARM.
- `--version` flag di main.go ditambahkan — keputusan saya (kecil, aditif) agar injeksi ldflags bisa diverifikasi `workflow --version`; user boleh menolak saat review.
- Release notes via `--generate-notes` — tanpa CHANGELOG.md manual (usulan saya, tidak ditolak user).
- Versioning: git tag = sumber kebenaran rilis; `internal/buildinfo.Version` = default dev (`0.2.0`); PRD `**Version:**` baris ikut di-sed oleh release.sh; README pakai badge shields github/v/release (auto-update).
- Actions di-bump ke major terbaru ber-Node 24 (`checkout@v7`, `setup-go@v7`, `upload-artifact@v7`, `download-artifact@v8`) — user melaporkan warning "Node.js 20 is deprecated" (2026-11-15).
- Script `scripts/release.sh` sengaja melakukan commit+tag+push dalam satu perintah — ini yang diminta user; agent tidak menjalankannya tanpa instruksi eksplisit.

## Blockers
- (kosong)

## Next
Menunggu approval user atas plan ini. Setelah disetujui: eksekusi Task 1–6 (Task 7 butuh approval terpisah untuk operasi git menulis). Pertanyaan eksekusi: apakah dikerjakan langsung di sesi ini, atau via delegasi?
