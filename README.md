# 🤖 Bot AI untuk Skrip & Narasi Video

<p align="center">
  <a href="https://go.dev/doc/install" target="_blank"><img src="https://img.shields.io/badge/Go-1.18%2B-00ADD8?style=for-the-badge&logo=go" alt="Versi Go"></a>
  <a href="LICENSE" target="_blank"><img src="https://img.shields.io/badge/Lisensi-MIT-green.svg?style=for-the-badge" alt="Lisensi"></a>
</p>

Sebuah bot Telegram fungsional yang dibangun dari awal menggunakan bahasa Go. Bot ini dapat membuat skrip video secara otomatis dan membuat narasi audio menggunakan ElevenLabs dan Google Gemini.

---

## 📚 Daftar Isi
- [✨ Fitur Utama](#-fitur-utama)
- [🚀 Panduan Memulai](#-panduan-memulai)
- [⚙️ Konfigurasi](#️-konfigurasi)

---

## ✨ Fitur Utama

- **🎬 Video menjadi Skrip**: Unggah video, dan bot akan menganalisis isinya untuk membuat skrip detail lengkap dengan penanda waktu (timestamp).
- **🎨 Berbagai Gaya Skrip**: Pilih gaya skrip "Profesional", "Naratif", atau masukkan gaya "Kustom" sesuai keinginan Anda.
- **🔄 Alur Kerja Interaktif**: Anda bisa menyetujui, membuat ulang, atau merevisi skrip yang dihasilkan AI dengan instruksi teks.
- **🔊 Teks menjadi Suara**: Ubah skrip final menjadi narasi audio berkualitas tinggi dengan berbagai pilihan suara dari ElevenLabs.
- **🗣️ Perintah Suara Langsung**: Gunakan perintah `/voice` untuk mengubah teks menjadi audio secara cepat tanpa harus mengunggah video.
- **🔐 Penanganan API yang Andal**: Dilengkapi fitur rotasi kunci API yang akan otomatis beralih ke kunci cadangan jika kunci utama kehabisan kuota.
- **💾 Penyimpanan Permanen**: Menggunakan database SQLite untuk menyimpan status pengguna, sehingga tidak ada data yang hilang saat bot di-restart.
- **🌐 Dukungan Multi-Bahasa**: Semua teks bot dikelola melalui file JSON (`en.json`, `id.json`) untuk kemudahan penerjemahan.
- **⚙️ Arsitektur Modular**: Struktur proyek yang bersih dan rapi, memisahkan logika AI, database, dan bot untuk kemudahan pemeliharaan.
- **📚 Perintah Ramah Pengguna**: Termasuk `/help`, `/cancel`, dan `/listvoices` untuk pengalaman pengguna yang lebih baik.

## 🚀 Panduan Memulai

Ikuti langkah-langkah berikut untuk menjalankan bot di komputer Anda.

### Persyaratan

- **Go** (versi 1.18 atau lebih tinggi)
- **C Compiler** (seperti `gcc`). Ini dibutuhkan oleh driver `go-sqlite3`.

---
#### 🔩 Panduan Instalasi Go

Jika Anda belum memiliki **Go** di sistem Anda, ikuti salah satu panduan di bawah ini.

##### 🐧 Linux (Ubuntu/Debian)
1.  Buka Terminal.
2.  Perbarui daftar paket Anda:
    ```bash
    sudo apt update
    ```
3.  Instal Go:
    ```bash
    sudo apt install golang
    ```

##### 📱 Termux (Android)
1.  Buka aplikasi Termux.
2.  Perbarui semua paket:
    ```bash
    pkg update && pkg upgrade
    ```
3.  Instal Go:
    ```bash
    pkg install golang
    ```

##### 🍎 macOS (via Homebrew)
1.  Buka Terminal.
2.  Jika Anda belum memiliki [Homebrew](https://brew.sh), instal terlebih dahulu.
3.  Instal Go menggunakan Homebrew:
    ```bash
    brew install go
    ```

---
##### ✅ Verifikasi Instalasi (Untuk Semua Sistem)
Setelah instalasi selesai, pastikan Go sudah terpasang dengan benar.

1.  Jalankan perintah ini di terminal Anda:
    ```bash
    go version
    ```
2.  Jika berhasil, Anda akan melihat output seperti ini (versi bisa berbeda):
    ```
    go version go1.22.5 linux/amd64
    ```
---

### Instalasi

1.  **Clone repositori ini:**
    ```bash
    git clone https://github.com/ilyaksco/video-script-bot.git
    cd video-script-bot
    ```

2.  **Buat dan atur file `.env`:**
    Salin file contoh untuk membuat file konfigurasi lokal Anda.
    ```bash
    cp .env.example .env
    ```
    Selanjutnya, buka file `.env` dan isi semua kunci API serta token Anda.

3.  **Install semua modul yang dibutuhkan:**
    Perintah ini akan mengunduh semua library yang diperlukan oleh proyek.
    ```bash
    go mod tidy
    ```

4.  **Jalankan bot:**
    Gunakan perintah berikut untuk menjalankan aplikasi. Flag `CGO_ENABLED=1` sangat penting agar driver SQLite dapat bekerja.
    ```bash
    CGO_ENABLED=1 go run main.go
    ```

Bot Anda sekarang seharusnya sudah berjalan dan terhubung ke Telegram!

## ⚙️ Konfigurasi

Semua pengaturan bot dikelola melalui file `.env` dan `voices.json`.

### File `.env`

- `TELEGRAM_BOT_TOKEN`: Token bot Anda dari @BotFather di Telegram.
- `GEMINI_API_KEYS`: Kunci API Google Gemini Anda. Anda bisa menambahkan beberapa kunci, dipisahkan dengan koma, untuk fitur rotasi otomatis.
- `ELEVENLABS_API_KEYS`: Kunci API ElevenLabs Anda. Juga mendukung beberapa kunci yang dipisahkan koma.
- `ELEVENLABS_MODEL_ID`: Model spesifik dari ElevenLabs yang ingin digunakan (contoh: `eleven_multilingual_v2`).
- `DEFAULT_LANG`: Bahasa default bot (`id` atau `en`).
- `DATABASE_PATH`: Lokasi file untuk database SQLite (contoh: `./bot_data.db`).

### File `voices.json`

File ini memungkinkan Anda untuk mengubah daftar suara yang tersedia tanpa harus mengubah kode. Cukup tambah atau hapus objek suara sesuai kebutuhan.
```json
{
  "voices": [
    {
      "voice_id": "21m00Tcm4TlvDq8ikWAM",
      "name": "Rachel"
    },
    {
      "voice_id": "AZnzlk1XvdvUeBnXmlld",
      "name": "Domi"
    }
  ]
}
