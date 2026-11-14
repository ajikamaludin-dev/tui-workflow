# Release Process — tui-workflow

Rilis dibuat dari **tag git**, bukan dari merge. Merge ke `main` hanya menjalankan CI; release benar-benar terbit saat tag `vX.Y.Z` di-push.

## Versi & Sumber Kebenaran

| Lokasi | Isi | Di-update oleh |
|---|---|---|
| git tag `vX.Y.Z` | Versi rilis (sumber kebenaran) | `scripts/release.sh` |
| `internal/buildinfo.Version` | Default dev (`0.2.0`) | `scripts/release.sh` (sed) |
| `PRD.md` `**Version:**` | Versi dokumen spec | `scripts/release.sh` (sed) |
| `PRD.md` Revision History | Riwayat per rilis | manual, sebelum release |
| README badge | `github/v/release` (auto-update) | otomatis |
| binary `workflow` | `buildinfo.Version` via ldflags dari tag | Makefile / CI |

Tidak ada `CHANGELOG.md`; release notes di-generate dari commit (`--generate-notes`).

## Alur Rilis

1. Semua fitur di-merge ke branch `dev` (PR → CI hijau).
2. Merge `dev` → `main` (CI jalan lagi).
3. Tambahkan baris Revision History di `PRD.md`:

   ```
   | v0.3.0 | YYYY-MM-DD | ringkasan perubahan |
   ```

   Commit & push baris ini — `scripts/release.sh` menolak berjalan tanpa baris tersebut.
4. Dari branch `main` dengan tree bersih:

   ```bash
   ./scripts/release.sh v0.3.0
   ```

   Script ini berurutan: validasi tag → cek branch `main` & tree bersih → cek tag belum ada → cek baris PRD ada → `go vet` + `go test -race` → sed `buildinfo/version.go` & `PRD.md` → commit `chore(release): vX.Y.Z` → tag anotasi → push `main` + tag.
5. Tag yang ter-push memicu workflow **Release** (`.github/workflows/release.yml`): test → build matrix `linux/amd64` + `linux/arm64` (CGO_ENABLED=0, cross-compile) → package `workflow_<ver>_linux_<arch>.tar.gz` + `checksums.txt` (sha256) → `gh release create --generate-notes`.
6. Verifikasi pasca-rilis:

   ```bash
   wget -qO- https://raw.githubusercontent.com/ajikamaludin-dev/tui-workflow/main/install.sh | sh
   workflow --version   # → tui-workflow v0.3.0
   ```

## Rollback Rilis

Jika workflow/release gagal atau salah:

```bash
gh release delete v0.3.0 --yes --cleanup-tag   # hapus release + tag remote
git tag -d v0.3.0                              # hapus tag lokal
# perbaiki masalah, lalu ulangi ./scripts/release.sh v0.3.0
```

Jika release sudah publik dan dipakai orang, jangan reuse nomor versi yang sama — terbitkan patch berikutnya (`v0.3.1`).

## Catatan

- Merge `dev` → `main` **tidak** membuat release; hanya tag.
- Binary release bernama `workflow` (bukan `tui-workflow`) — asset `workflow_<ver>_linux_<arch>.tar.gz`.
- Checksum wajib diverifikasi `install.sh` sebelum binary diganti; binary lama tidak disentuh bila verifikasi gagal.
