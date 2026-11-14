# LSP Audit Checklist — `internal/lsp` (Hand-rolled JSON-RPC 2.0 over stdio)

> **Status:** v1-acceptable, **known debt**. LSP client ini ditulis tangan karena kebutuhan spesifik proyek (no heavy LSP framework). File ini adalah checklist audit mandiri — update setiap kali klien LSP diubah.

## 1. Position Encoding (UTF-16 vs Byte) — PALING SERING SALAH
- [ ] `Position.Character` di request/response diperlakukan sebagai **UTF-16 code units** (default LSP), bukan byte offset atau rune index — periksa konversi dari buffer (`[][]rune` di `internal/editor`) ke `Position`.
- [ ] Konversi `byte → UTF-16 units` mempertimbangkan surrogate pairs (rune > 0xFFFF = 2 units).
- [ ] `TextEdit` dari server (range → newText) diterapkan dengan offset UTF-16 yang benar terhadap baris rune-based.
- [ ] `Range` pada diagnostics dipetakan ke baris/kolom buffer dengan encoding yang sama.
- [ ] Multi-cursor position updates konsisten dengan encoding ini.

## 2. Document Synchronization
- [ ] `didOpen` dikirim sekali dengan `textDocument/didOpen` (full text).
- [ ] `didChange` — apakah klien memakai **incremental sync** (`TextDocumentSyncKind = 2`) atau full-text (`= 1`)? Jika incremental: `TextDocumentContentChangeEvent.range` dan `rangeLength` WAJIB diisi akurat; jika salah → korupsi buffer server-side yang sulit di-debug.
- [ ] `didSave` dikirim hanya jika server meminta (`capabilities.textDocumentSync.save`).
- [ ] `didClose` dikirim saat tab ditutup (penting untuk memory server).
- [ ] Open-file race: edit yang terjadi SEBELUM server siap (`initialized`) — antrian atau full-sync ulang.

## 3. Capability Negotiation
- [ ] `initialize` request hanya dikirim sekali per server process.
- [ ] Response `capabilities` dibaca **defensively** — semua field opsional; server boleh menolak fitur (mis. completion, definition, references).
- [ ] Klien tidak memakai fitur yang ditolak (cek `capabilities.completionProvider != nil` sebelum `textDocument/completion`).
- [ ] `initialized` notification dikirim setelah response `initialize` diterima.
- [ ] `shutdown` → `exit` dikirim saat server ditutup (jangan langsung kill process).

## 4. Robustness & Error Recovery
- [ ] Server crash → klien tidak deadlock (timeout pada semua request) dan tidak panic (nil response).
- [ ] `stderr` dari server LSP di-log terpisah (jangan dianggap response).
- [ ] Content-Length header diparsing dengan benar (base-10 integer; `\r\n\r\n` terminator).
- [ ] Response `id` yang tidak cocok diabaikan (server boleh mengirim notifikasi duluan).
- [ ] Request tanpa response di-cancel saat dokumen ditutup / server mati.
- [ ] Concurrent requests: satu stdio reader loop; pending request map dengan `id → chan`.

## 5. Known Limitations (diterima untuk v1)
- [ ] Incremental document sync — jika belum ada, dokumentasikan fallback full-text dan risiko performa pada file besar.
- [ ] Position encoding — jika belum diverifikasi UTF-16, dokumentasikan bias byte-vs-utf16 (bug paling halus: cursor/kolom bergeser di baris dengan CJK/emoji).
- [ ] Capability negotiation — jika belum defensif, server yang menolak fitur bisa menyebabkan request gantung.
- [ ] Tidak ada `$/cancelRequest` — request tidak dibatalkan saat server lambat.
- [ ] Tidak ada workspace folders management (multi-root workspace tidak didukung secara native).

## Insiden Historis & Catatan
- LSP completion/gofmt/lint bridges pernah menjadi sumber bug sulit: (1) `.lsp_unavailable` fallback ke In-Memory Symbol Index (Path B) adalah solusi yang disengaja — jangan dihapus.
- Jika ditemukan bug encoding/sync: **perbaiki dengan test unit** di `internal/lsp/protocol_test.go` sebelum menyentuh kode lain.
