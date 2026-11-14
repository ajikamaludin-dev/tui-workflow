
## Catatan kritis (jujur)

**1. `internal/app` mulai menjadi God Object.** `update_keys.go` 1.503 baris, `model.go` 1.144, `update_mouse.go` 970, `palette_actions.go` 714 — ~5.300 baris dalam satu package. Elm Architecture memang membuat `Update()` itu besar, tapi saat ini satu fungsi menangani puluhan modal dan mode. Ini titik yang paling berisiko membusuk; pemecahan per-domain handler (yang sudah dimulai dengan `dialog_handlers.go`, `navigation.go`, `resize.go`) perlu diperluas.

**2. Ketergantungan pada ekosistem Charmbracelet yang bergerak cepat.** Bubble Tea v2 sudah rilis; Anda di v1.3.4. Setiap kenaikan versi ekosistem ini bisa menyakitkan, apalagi dengan hit-testing mouse manual yang sensitif.

**3. LSP client hand-rolled** adalah area dengan bug paling halus (position encoding, incremental sync, capability negotiation). Untuk v1 cukup, tapi ini akan jadi sumber bug yang sulit di-debug.

## Penilaian keseluruhan

1. **Kedalaman > keluasan** — selesaikan bug di TODO.md, stabilkan terminal & git karena itu pembeda utama Anda
2. **Sinkronkan PRD dengan realitas** — naikkan ke v0.2.0 yang jujur mencerminkan AI/terminal/multicursor, atau Anda kehilangan "sumber kebenaran" yang justru jadi kekuatan metodologi Anda
3. **Refactor `internal/app` secara bertahap** sebelum ia tumbuh melebihi 6–7k baris dan buat rule agar dipatuhi ai agent agar tidak membuat god class
