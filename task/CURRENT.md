---
title: Git Sidebar & Workflow Fixes
status: review
started: 2026-11-12
completed:
branch:
priority: high
---

# Git Sidebar & Workflow Fixes

## Ground Truth Findings
1. **Commit Input Prompt**: Diverifikasi di `internal/git/view.go:70-76`: `ti := textinput.New()` memuat prompt bawaan `"> "`. Perlu diatur `ti.Prompt = ""` agar karakter `>` hilang.
2. **Commit & Push Buttons**: Diverifikasi di `internal/git/view.go:410-425`: tombol dirender berdampingan statis `[ Commit ] [ Push ]` dengan kolom hardcoded (cols 1..8 dan 9..16). TODO mensyaratkan tombol fullwidth, warna biru accent yang sama, dan bergantian: jika ada perubahan file muncul `[ Commit ]`, jika working tree bersih dan terdapat unpushed commit muncul `[ Push ]`.
3. **Stage All / Unstage All**: Diverifikasi di `internal/git/view.go:428-500` dan `internal/git/repo.go:137-181`: saat ini aksi `[+]` dan `[-]` hanya tersedia per-item file. Header `Staged Changes` dan `Changes` belum memiliki tombol header `[-]` (unstage all) dan `[+]` (stage all). `Repo` belum memiliki method `StageAll()` dan `UnstageAll()`.
4. **Git View Scrolling**: Diverifikasi di `internal/git/view.go:37-38, 563-568`: struct `GitView` telah memiliki field `ScrollOffset int`, tetapi `Render()` memotong baris secara mentah `lines = lines[:height]` tanpa memperhitungkan `ScrollOffset`. Navigasi arrow/j/k dan mouse wheel belum mengatur offset scrolling.
5. **Sidebar Mouse Wheel**: Diverifikasi di `internal/app/model.go:730-760`: event `tea.MouseButtonWheelUp` dan `tea.MouseButtonWheelDown` hanya menangani editor, markdown viewer, dan diff viewer. Ketika mouse berada di atas sidebar (`msg.X < m.SidebarWidth`), event scroll wheel diabaikan, menyebabkan file explorer dan git view tidak bisa di-scroll dengan mouse wheel.
6. **Git Remote & Unpushed Detection**: Diverifikasi di `internal/git/repo.go:65-77`: `Repo.Push()` sudah ada, namun belum ada pengecekan komparasi antara commit `HEAD` dan remote tracking branch (`refs/remotes/origin/<branch>`) untuk mengetahui status unpushed commit.

## Objective & Approach
Menyelesaikan paket perbaikan Git Sidebar & Scrolling sesuai spesifikasi `TODO.md`:
1. Membersihkan input commit message dari karakter `>`.
2. Menyediakan tombol fullwidth interaktif yang bergantian secara dinamis antara `[ Commit ]` dan `[ Push ]` sesuai ketersediaan perubahan file dan unpushed commit.
3. Menambahkan status loading asinkron / visual feedback saat commit atau push sedang diproses.
4. Menambahkan tombol header `[-]` untuk unstage all di Staged Changes dan `[+]` untuk stage all di Changes.
5. Mengimplementasikan viewport scrolling penuh (keyboard arrow/j/k dan mouse wheel) pada Git View.
6. Menambahkan penanganan mouse wheel scroll pada File Explorer tree di sidebar.

## Terminal & Layout Impact Analysis
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| Kolom 1 (Sidebar: Git View) | Tombol fullwidth dan tombol header `[+]`/`[-]` berisiko overflow melebihi lebar sidebar (28 kolom). | Gunakan `ansi.Truncate` dan hitung lebar tombol dinamis berbasis `width - 2`. Batasi tinggi baris tetap 1 baris. |
| Kolom 1 (Sidebar: Git View) | Render baris yang melebihi tinggi terminal dapat merusak scrolling jika `ScrollOffset` tidak diklem. | Batasi `ScrollOffset` agar `maxScroll = max(0, len(lines) - height)`. Potong tepat `lines[ScrollOffset : ScrollOffset+height]`. |
| Kolom 1 (Sidebar: Files Tree) | Mouse wheel scroll berpotensi melewati batas jumlah node tree. | Panggil `t.adjustScroll()` dan batasi `ScrollOffset` antara 0 dan `len(t.VisibleNodes) - height`. |

## UI / Keymap Flow
1. **Commit Input**: Fokus pada kolom input pesan commit tampil bersih tanpa awalan `>`.
2. **Tombol Aksi Git**:
   - Jika ada file di Changes atau Staged: tampilkan tombol fullwidth `[ Commit ]` (Background biru/Accent).
   - Jika working tree bersih (`len(Staged) == 0 && len(Changes) == 0`) dan ada commit yang belum di-push: tampilkan tombol fullwidth `[ Push (N) ]` (Background biru/Accent).
   - Saat proses commit atau push berjalan: tampilkan state loading `[ Committing... ]` atau `[ Pushing... ]`.
3. **Stage All / Unstage All**:
   - Header Staged Changes: `▼ Staged Changes (N)        [-]` -> klik `[-]` meng-unstage semua file.
   - Header Changes: `▼ Changes (N)               [+]` -> klik `[+]` meng-stage semua file.
4. **Scrolling Sidebar**:
   - Scroll wheel up/down saat kursor mouse di sidebar: melakukan scroll pada File Explorer (jika tab Files aktif) atau Git View (jika tab Git aktif).
   - Navigasi keyboard `Up`/`Down`/`j`/`k` pada Git View secara otomatis menyesuaikan `ScrollOffset` agar item yang dipilih selalu tampak di viewport.

## Files Affected & Execution Steps
1. `internal/git/repo.go`:
   - Tambahkan method `StageAll() error` untuk menambahkan seluruh file di working tree ke staging index.
   - Tambahkan method `UnstageAll() error` untuk me-reset staging index ke HEAD.
   - Tambahkan method `GetUnpushedCount() (int, error)` untuk menghitung commit lokal yang belum ada di remote branch.
2. `internal/git/view.go`:
   - Set `ti.Prompt = ""` pada `NewGitView()`.
   - Tambahkan state `IsLoading bool`, `LoadingMsg string`, dan `UnpushedCount int`.
   - Tambahkan target klik `ClickStageAllAction` dan `ClickUnstageAllAction` pada `GitClickTarget`.
   - Update `Render()`: render tombol fullwidth dinamis (Commit vs Push), tombol `[+]`/`[-]` di header seksi, dan viewport slice `lines[ScrollOffset : ScrollOffset+height]`.
   - Sesuaikan `HandleMouseClick()` dengan koordinat offset scrolling dan aksi stage/unstage all.
   - Tambahkan method `ScrollUp(delta int)` dan `ScrollDown(delta int, height int)`.
   - Perbarui navigasi keyboard di `Update()` untuk memanggil penyesuaian scroll otomatis (`ensureVisible()`).
3. `internal/app/model.go`:
   - Tangani `tea.MouseButtonWheelUp` dan `tea.MouseButtonWheelDown` di area sidebar (`msg.X < m.SidebarWidth` dan `msg.Y >= 2`).
   - Sambungkan aksi mouse click dan keyboard commit/push dengan state loading.
4. `internal/explorer/tree.go`:
   - Tambahkan method `ScrollUp(delta int)` dan `ScrollDown(delta int, height int)` untuk dipanggil saat mouse wheel scroll di sidebar files.
5. `internal/git/repo_test.go` & `internal/app/features_test.go`:
   - Tambahkan pengujian unit untuk `StageAll`, `UnstageAll`, `GetUnpushedCount`, tombol fullwidth, scrolling git view, dan mouse wheel sidebar.

## Tasks
- [x] Implementasi `StageAll`, `UnstageAll`, `GetUnpushedCount` di `internal/git/repo.go`
- [x] Update `internal/git/view.go` (hapus prompt `>`, tombol fullwidth bergantian, header `[+]`/`[-]`, viewport scrolling)
- [x] Implementasi `ScrollUp` & `ScrollDown` di `internal/explorer/tree.go`
- [x] Wiring mouse wheel sidebar dan loading feedback di `internal/app/model.go`
- [x] Update pengujian di `internal/git/repo_test.go`, buat `internal/git/view_test.go` dan `internal/app/sidebar_scroll_test.go`
- [x] Verification: `go vet ./...` ✓, `go test -v -race ./...` ✓

## Manual Test Checklist
- [ ] Buka tab Git (`Alt+2`): pastikan tidak ada karakter `>` pada box input commit.
- [ ] Buat beberapa file modifikasi: pastikan tombol bertuliskan `[ Commit ]` fullwidth warna biru.
- [ ] Klik `[+]` di header `Changes`: semua file berpindah ke `Staged Changes`.
- [ ] Klik `[-]` di header `Staged Changes`: semua file kembali ke `Changes`.
- [ ] Lakukan commit: verifikasi feedback loading dan tombol berganti ke `[ Push ]` jika ada unpushed commit.
- [ ] Buka repo dengan banyak file di Git/Files: gunakan mouse wheel untuk scroll ke atas dan bawah; gunakan arrow key untuk memastikan kursor tetap di viewport.

## Acceptance Criteria
- [x] Tidak ada karakter `>` pada commit message input.
- [x] Tombol Commit dan Push berpenampilan fullwidth dengan warna biru (Accent).
- [x] Tombol Commit dan Push hanya tampil bergantian sesuai kondisi file changes dan unpushed commits.
- [x] Header Staged Changes memiliki tombol `[-]` (unstage all) dan Changes memiliki `[+]` (stage all) yang berfungsi via mouse click.
- [x] Git view dapat di-scroll menggunakan mouse wheel dan arrow keys.
- [x] File explorer dapat di-scroll menggunakan mouse wheel.
- [x] `go vet ./...` 0 error, `go test -v -race ./...` 100% PASS bebas data race.

## Improvement 1 (2026-11-12 — Asynchronous Push & Commit with UI Loading Feedback)

### Analisis Fakta
- Sebelumnya pemanggilan `git.Push()` dan `git.Commit()` di `internal/app/model.go` berjalan secara sinkron di method `Update()`.
- Karena proses push berinteraksi dengan remote git server melalui network, pemanggilan sinkron memblokir event loop Bubble Tea sehingga status loading tidak sempat dirender ke layar sebelum eksekusi selesai.

### Rencana Perbaikan
1. Ubah operasi push dan commit menjadi command asinkron Bubble Tea (`tea.Cmd`): `gitPushCmd(repo)` dan `gitCommitCmd(repo, msg)`.
2. Saat tombol diklik: aktifkan `IsLoading = true`, `LoadingMsg = "Pushing..."` atau `"Committing..."`, tampilkan status toast, dan kembalikan command ke Bubble Tea agar UI segera me-render state loading.
3. Tangani hasil respon asinkron pada message handler `gitPushResultMsg` dan `gitCommitResultMsg` di `Update()`: reset `IsLoading = false`, perbarui status bar dan view.

### Tasks
- [x] Definisikan `gitPushCmd`, `gitCommitCmd`, `gitPushResultMsg`, dan `gitCommitResultMsg` di `internal/app/model.go`
- [x] Hubungkan event klik mouse dan keyboard `ctrl+enter` dengan command asinkron dan state `IsLoading`
- [x] Update test `TestGitCommitAndPushButtonInteraction` di `internal/app/features_test.go`
- [x] Verifikasi: `go vet ./...` ✓, `go test -v -race -count=1 ./...` ✓

## Decisions
- **Tombol Bergantian**: Tombol Commit diprioritaskan selama ada perubahan di Staged atau Changes. Jika working tree bersih, tombol beralih ke Push bila terdapat commit yang belum dipublikasikan ke remote.
- **Unstage All Implementation**: Menggunakan `worktree.Reset` dengan `MixedReset` untuk mengembalikan index ke commit HEAD secara aman tanpa menyentuh file fisik di disk.
- **Asynchronous Git Operations**: Semua interaksi jaringan (push) dan disk I/O (commit) dijalankan via goroutine background `tea.Cmd` agar UI tetap responsif 60 FPS dan feedback loading tampil seketika.

## Blockers
- None.

## Next
- Tunggu konfirmasi pengujian manual dari user sebelum arsipkan task ke `task/done/`.
