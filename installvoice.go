package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// `read-e --install-voice` downloads Piper (a small offline neural text-to-speech
// engine) and one English voice into the user's data directory. Every file is
// pinned by SHA-256, so only exactly these artifacts are ever accepted.
const (
	piperURL    = "https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_x86_64.tar.gz"
	piperSHA256 = "a50cb45f355b7af1f6d758c1b360717877ba0a398cc8cbe6d2a7a3a26e225992"

	voiceName      = "en_US-lessac-medium"
	voiceBaseURL   = "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/"
	voiceModelSHA  = "5efe09e69902187827af646e1a6e9d269dee769f9877d17b16b1b46eeaaf019f"
	voiceConfigSHA = "efe19c417bed055f2d69908248c6ba650fa135bc868b0e6abb3da181dab690a0"
)

// speechDataDir is where the natural voice lives (~/.local/share/read-e by default).
func speechDataDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "read-e")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "read-e")
	}
	return filepath.Join(home, ".local", "share", "read-e")
}

// downloadVerified fetches url into dest and only keeps it if its SHA-256 matches.
func downloadVerified(url, dest, wantSHA string, progress func(done, total int64)) error {
	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	h := sha256.New()
	var done int64
	buf := make([]byte, 128<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				tmp.Close()
				return werr
			}
			done += int64(n)
			if progress != nil {
				progress(done, resp.ContentLength)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			tmp.Close()
			return rerr
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != wantSHA {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", url, got, wantSHA)
	}
	return os.Rename(tmp.Name(), dest)
}

// extractTarGz unpacks a .tar.gz into dest, refusing paths or links that would
// land outside it.
func extractTarGz(path, dest string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	root, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	inside := func(p string) bool {
		return p == root || strings.HasPrefix(p, root+string(os.PathSeparator))
	}

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(root, hdr.Name)
		if !inside(target) {
			return fmt.Errorf("unsafe path in archive: %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o755|0o600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			resolved := hdr.Linkname
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(target), resolved)
			}
			if filepath.IsAbs(hdr.Linkname) || !inside(filepath.Clean(resolved)) {
				return fmt.Errorf("unsafe link in archive: %q -> %q", hdr.Name, hdr.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		default:
			// ignore anything else (devices, fifos, ...)
		}
	}
}

func printProgress(label string) func(done, total int64) {
	last := -10
	return func(done, total int64) {
		if total <= 0 {
			return
		}
		pct := int(done * 100 / total)
		if pct >= last+10 || pct == 100 {
			last = pct
			fmt.Printf("  %s: %3d%% (%d / %d MB)\n", label, pct, done>>20, total>>20)
		}
	}
}

// installVoice implements `read-e --install-voice`; it returns the process exit code.
func installVoice() int {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		fmt.Fprintln(os.Stderr, "The natural voice installer supports Linux x86-64 only.")
		return 1
	}
	dir := speechDataDir()
	voices := filepath.Join(dir, "voices")
	if err := os.MkdirAll(voices, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "cannot create", voices+":", err)
		return 1
	}
	fmt.Println("Installing a natural reading voice into", dir)

	// 1. Piper engine
	fmt.Println("[1/3] Downloading the Piper speech engine (~25 MB)")
	archive := filepath.Join(dir, "piper.tar.gz")
	if err := downloadVerified(piperURL, archive, piperSHA256, printProgress("piper")); err != nil {
		fmt.Fprintln(os.Stderr, "failed:", err)
		return 1
	}
	defer os.Remove(archive)

	staging := filepath.Join(dir, ".piper-staging")
	_ = os.RemoveAll(staging)
	if err := extractTarGz(archive, staging); err != nil {
		fmt.Fprintln(os.Stderr, "failed to unpack:", err)
		_ = os.RemoveAll(staging)
		return 1
	}
	_ = os.RemoveAll(filepath.Join(dir, "piper"))
	if err := os.Rename(filepath.Join(staging, "piper"), filepath.Join(dir, "piper")); err != nil {
		fmt.Fprintln(os.Stderr, "failed to install:", err)
		return 1
	}
	_ = os.RemoveAll(staging)

	// 2. Voice
	fmt.Println("[2/3] Downloading the voice,", voiceName, "(~65 MB)")
	if err := downloadVerified(voiceBaseURL+voiceName+".onnx.json", filepath.Join(voices, voiceName+".onnx.json"), voiceConfigSHA, nil); err != nil {
		fmt.Fprintln(os.Stderr, "failed:", err)
		return 1
	}
	if err := downloadVerified(voiceBaseURL+voiceName+".onnx", filepath.Join(voices, voiceName+".onnx"), voiceModelSHA, printProgress("voice")); err != nil {
		fmt.Fprintln(os.Stderr, "failed:", err)
		return 1
	}

	// 3. Make sure it actually produces speech
	fmt.Println("[3/3] Testing")
	setup, ok := findPiper()
	if !ok {
		fmt.Fprintln(os.Stderr, "installed, but the voice could not be located")
		return 1
	}
	cmd := exec.Command(setup.bin, "--model", setup.model, "--config", setup.config, "--output-raw")
	cmd.Stdin = strings.NewReader("Reading aloud is ready.")
	out, err := cmd.Output()
	if err != nil || len(out) < 4000 {
		fmt.Fprintln(os.Stderr, "the voice engine did not produce audio:", err)
		return 1
	}
	if _, found := setup.player(); !found {
		fmt.Println("Note: no audio player found; install one (e.g. sudo apt install alsa-utils) to hear it.")
	}
	fmt.Println("Done. Restart read e, then use the speaker button in an EPUB.")
	return 0
}
