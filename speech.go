package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The embedded web view (WebKitGTK) often ships without the browser speech API,
// so read aloud drives a system speech program instead.
//
// Engines in order of preference:
//   - piper    (natural offline neural voice; installed with `read-e --install-voice`)
//   - spd-say  (speech-dispatcher; honours the user's configured voice/language)
//   - espeak-ng, espeak (direct)
var speechEngines = []string{"spd-say", "espeak-ng", "espeak"}

// piperSetup describes an installed Piper engine and voice.
type piperSetup struct {
	bin, model, config string
	sampleRate         int
}

// findPiper locates the Piper engine and a voice installed by --install-voice.
func findPiper() (piperSetup, bool) {
	dir := speechDataDir()
	bin := filepath.Join(dir, "piper", "piper")
	if st, err := os.Stat(bin); err != nil || st.IsDir() {
		return piperSetup{}, false
	}
	models, _ := filepath.Glob(filepath.Join(dir, "voices", "*.onnx"))
	sort.Strings(models)
	for _, model := range models {
		config := model + ".json"
		raw, err := os.ReadFile(config)
		if err != nil {
			continue
		}
		var cfg struct {
			Audio struct {
				SampleRate int `json:"sample_rate"`
			} `json:"audio"`
		}
		if json.Unmarshal(raw, &cfg) != nil || cfg.Audio.SampleRate == 0 {
			cfg.Audio.SampleRate = 22050
		}
		return piperSetup{bin, model, config, cfg.Audio.SampleRate}, true
	}
	return piperSetup{}, false
}

// player returns the command that plays raw 16-bit mono PCM from stdin.
func (p piperSetup) player() ([]string, bool) {
	rate := strconv.Itoa(p.sampleRate)
	candidates := [][]string{
		{"pw-cat", "-p", "--rate", rate, "--channels", "1", "--format", "s16", "-"},
		{"paplay", "--raw", "--rate=" + rate, "--channels=1", "--format=s16le"},
		{"aplay", "-q", "-t", "raw", "-r", rate, "-f", "S16_LE", "-c", "1", "-"},
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", "-f", "s16le", "-ar", rate, "-ac", "1", "-i", "-"},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err == nil {
			return c, true
		}
	}
	return nil, false
}

// Speed is a multiplier (1 = normal), volume a percentage (100 = normal).
const (
	minSpeechSpeed, maxSpeechSpeed   = 0.5, 2.5
	minSpeechVolume, maxSpeechVolume = 10, 150
)

func normalizeSpeed(speed float64) float64 {
	if speed <= 0 {
		return 1
	}
	return math.Min(maxSpeechSpeed, math.Max(minSpeechSpeed, speed))
}

func normalizeVolume(volume int) int {
	if volume <= 0 {
		return 100
	}
	if volume < minSpeechVolume {
		return minSpeechVolume
	}
	if volume > maxSpeechVolume {
		return maxSpeechVolume
	}
	return volume
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// speechCommand builds the command line that speaks text with a simple engine.
// The text follows "--" so a chunk beginning with "-" is not read as an option.
func speechCommand(engine, text string, speed float64, volume int) []string {
	speed = normalizeSpeed(speed)
	volume = normalizeVolume(volume)
	switch engine {
	case "spd-say":
		rate := clampInt(int(math.Round((speed-1)*100)), -100, 100) // -100..100, 0 = normal
		vol := clampInt((volume-50)*2, -100, 100)                   // 100% => +100 (these engines are quiet)
		return []string{engine, "-w", "-r", strconv.Itoa(rate), "-i", strconv.Itoa(vol), "--", text}
	default:
		wpm := clampInt(int(math.Round(165*speed)), 80, 450)
		amp := clampInt(volume*2, 0, 200) // 100% => 200, espeak's maximum
		return []string{engine, "-s", strconv.Itoa(wpm), "-a", strconv.Itoa(amp), "--", text}
	}
}

// applyGain scales 16-bit little-endian PCM samples in place (clamping at full scale).
func applyGain(pcm []byte, gain float64) {
	if gain == 1 {
		return
	}
	for i := 0; i+1 < len(pcm); i += 2 {
		v := float64(int16(binary.LittleEndian.Uint16(pcm[i:]))) * gain
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		binary.LittleEndian.PutUint16(pcm[i:], uint16(int16(v)))
	}
}

// copyWithGain streams PCM from src to dst applying gain, then closes dst. Reads can
// split a 16-bit sample, so a trailing odd byte is carried to the next read.
func copyWithGain(dst io.WriteCloser, src io.Reader, gain float64) {
	defer dst.Close()
	buf := make([]byte, 32<<10)
	var carry []byte
	for {
		n, err := src.Read(buf)
		if n > 0 {
			data := append(carry, buf[:n]...)
			even := len(data) &^ 1
			carry = append([]byte(nil), data[even:]...)
			chunk := data[:even]
			applyGain(chunk, gain)
			if _, werr := dst.Write(chunk); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func lookupSpeechEngine() string {
	if p, ok := findPiper(); ok {
		if _, hasPlayer := p.player(); hasPlayer {
			return "piper"
		}
	}
	for _, name := range speechEngines {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

// prefetched is a chunk of speech synthesized ahead of time.
type prefetched struct {
	text  string
	speed float64
	done  chan struct{}
	data  []byte
	err   error
}

// synthesizePiper renders text to raw PCM with Piper.
func synthesizePiper(setup piperSetup, text string, speed float64) ([]byte, error) {
	cmd := piperSynthCommand(setup, speed)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Output()
}

// piperTiming maps a speed multiplier to Piper's length scale and sentence silence.
// Using 1/speed directly makes "2x" only about 1.6x faster: pauses and the model's
// minimum phoneme length do not shrink with it. This is fitted to measured output
// (within ~10% of true speed up to 2x) and is exactly neutral at speed 1.
func piperTiming(speed float64) (lengthScale, sentenceSilence float64) {
	speed = normalizeSpeed(speed)
	return math.Max(0.3, 1.28/speed-0.28), 0.2 / speed
}

// piperSynthCommand builds the synthesizer for the given speed.
func piperSynthCommand(setup piperSetup, speed float64) *exec.Cmd {
	scale, silence := piperTiming(speed)
	return exec.Command(setup.bin, "--model", setup.model, "--config", setup.config, "--output-raw",
		"--length_scale", strconv.FormatFloat(scale, 'f', 3, 64),
		"--sentence_silence", strconv.FormatFloat(silence, 'f', 3, 64))
}

// PrefetchSpeech starts synthesizing the chunk that will be read next, so there
// is no pause while the voice reloads. It does nothing for other engines.
func (a *App) PrefetchSpeech(text string, speed float64) {
	speed = normalizeSpeed(speed)
	if text == "" || lookupSpeechEngine() != "piper" {
		return
	}
	setup, ok := findPiper()
	if !ok {
		return
	}
	a.speechMu.Lock()
	if a.speechNext != nil && a.speechNext.text == text && a.speechNext.speed == speed {
		a.speechMu.Unlock()
		return
	}
	pf := &prefetched{text: text, speed: speed, done: make(chan struct{})}
	a.speechNext = pf
	a.speechMu.Unlock()

	go func() {
		pf.data, pf.err = synthesizePiper(setup, text, speed)
		close(pf.done)
	}()
}

// takePrefetched returns the prefetched audio for text, if there is any.
func (a *App) takePrefetched(text string, speed float64) *prefetched {
	a.speechMu.Lock()
	defer a.speechMu.Unlock()
	pf := a.speechNext
	if pf == nil || pf.text != text || pf.speed != speed {
		return nil
	}
	a.speechNext = nil
	return pf
}

// SpeechEngine reports which speech program read aloud will use ("" if none).
// It is looked up each time so installing a voice takes effect without a restart.
func (a *App) SpeechEngine() string {
	return lookupSpeechEngine()
}

// Speak reads text aloud and returns when it has finished or was stopped.
// speed is a multiplier (1 = normal) and volume a percentage (100 = normal); zero
// values mean "normal".
func (a *App) Speak(text string, speed float64, volume int) error {
	engine := a.SpeechEngine()
	if engine == "" {
		return fmt.Errorf("no speech engine installed")
	}
	if text == "" {
		return nil
	}
	speed = normalizeSpeed(speed)
	volume = normalizeVolume(volume)

	var cmds []*exec.Cmd
	var copyDone chan struct{} // closed when the gain stage has passed everything on
	if engine == "piper" {
		setup, _ := findPiper()
		playerArgs, _ := setup.player()
		play := exec.Command(playerArgs[0], playerArgs[1:]...)
		gain := float64(volume) / 100

		if pf := a.takePrefetched(text, speed); pf != nil {
			<-pf.done
			if pf.err == nil && len(pf.data) > 0 {
				// Already synthesized while the previous chunk was playing
				pcm := append([]byte(nil), pf.data...)
				applyGain(pcm, gain)
				play.Stdin = bytes.NewReader(pcm)
				cmds = []*exec.Cmd{play}
			}
		}
		if cmds == nil {
			synth := piperSynthCommand(setup, speed)
			synth.Stdin = strings.NewReader(text)
			pipe, err := synth.StdoutPipe()
			if err != nil {
				return err
			}
			if gain == 1 {
				play.Stdin = pipe
			} else {
				in, err := play.StdinPipe()
				if err != nil {
					return err
				}
				copyDone = make(chan struct{})
				go func() {
					copyWithGain(in, pipe, gain)
					close(copyDone)
				}()
			}
			cmds = []*exec.Cmd{synth, play}
		}
	} else {
		argv := speechCommand(engine, text, speed, volume)
		cmds = []*exec.Cmd{exec.Command(argv[0], argv[1:]...)}
	}

	a.speechMu.Lock()
	a.speechCmds = cmds
	gen := a.speechGen
	a.speechMu.Unlock()

	// Start the player first so it is ready to consume the synthesizer's output
	var err error
	for i := len(cmds) - 1; i >= 0; i-- {
		if serr := cmds[i].Start(); serr != nil {
			err = serr
			break
		}
	}
	if err == nil {
		// Wait must not run before every read from the synthesizer's pipe is done
		if copyDone != nil {
			<-copyDone
		}
		for _, c := range cmds {
			if werr := c.Wait(); werr != nil && err == nil {
				err = werr
			}
		}
	} else {
		for _, c := range cmds {
			if c.Process != nil {
				_ = c.Process.Kill()
				_ = c.Wait()
			}
		}
		if copyDone != nil {
			<-copyDone
		}
	}

	a.speechMu.Lock()
	interrupted := a.speechGen != gen
	if len(a.speechCmds) > 0 && a.speechCmds[0] == cmds[0] {
		a.speechCmds = nil
	}
	a.speechMu.Unlock()

	if interrupted {
		return nil // stopped on purpose; a killed process is not an error
	}
	return err
}

// StopSpeaking interrupts whatever is being read.
func (a *App) StopSpeaking() {
	a.speechMu.Lock()
	cmds := a.speechCmds
	a.speechGen++ // marks any Speak in flight as interrupted
	a.speechNext = nil
	a.speechMu.Unlock()

	if lookupSpeechEngine() == "spd-say" {
		// Killing the client is not enough: the speech-dispatcher daemon keeps speaking
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = exec.CommandContext(ctx, "spd-say", "-C").Run()
	}
	for _, c := range cmds {
		if c.Process != nil {
			_ = c.Process.Kill()
		}
	}
}
