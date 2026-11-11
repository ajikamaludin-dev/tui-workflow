---
title: # Nama task (sesuai objective utama)
status: # planning | plan_approved | in_progress | blocked | review | done
started: # YYYY-MM-DD
completed: # YYYY-MM-DD (diisi saat pindah ke done/)
branch: # branch git (jika ada)
priority: # high | medium | low
---

# {Title}

## Objective
{1-2 kalimat deskripsi tujuan task.}

## Plan
{Pendekatan teknis + daftar file yang akan dibuat/diubah. Sajikan ke user & tunggu approval sebelum coding.}

## Terminal & Layout Impact Analysis
{Analisis dampak ke 4 zona layout, lebar terminal, ANSI handling, atau status bar. Jika tidak ada, tulis "No layout impact."}
| Affected Zone | Risk | Mitigation |
| ------------- | ---- | ---------- |
| | | |

## UI / Keymap Flow
{Alur interaksi pengguna, keybinding baru/diubah, respons mouse, dan perubahan tampilan teks/ASCII.}

## Tasks
- [ ] {Task 1}
- [ ] {Task 2}
- [ ] Verification: go vet ./... ✓, go test -v -race ./... ✓

## Manual Test Checklist
- [ ] {Langkah pengujian manual oleh user di terminal — aksi + hasil yang diharapkan}

## Acceptance Criteria
- [ ] {Kondisi terverifikasi — task belum selesai sampai semua kriteria tercentang}

## Decisions
- {Keputusan teknis} — {Alasan / trade-off}

## Blockers
- {Kendala} — {Solusi sementara bila ada}

## Next
{Langkah eksplisit berikutnya untuk sesi selanjutnya.}
