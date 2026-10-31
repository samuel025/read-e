package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// speechCommand builds the command line that speaks text with a simple engine.
// The text follows "--" so a chunk beginning with "-" is not read as an option.
// Volume is raised: the engines' defaults are quiet.
func speechCommand(engine, text string) []string {
	switch engine {
	case "spd-say":
		return []string{engine, "-w", "-i", "100", "--", text} // -w: return when the message is done
	default:
		return []string{engine, "-s", "165", "-a", "200", "--", text}
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
	text string
	done chan struct{}
	data []byte
	err  error
}

// synthesizePiper renders text to raw PCM with Piper.
func synthesizePiper(setup piperSetup, text string) ([]byte, error) {
	cmd := exec.Command(setup.bin, "--model", setup.model, "--config", setup.config, "--output-raw")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Output()
}

// PrefetchSpeech starts synthesizing the chunk that will be read next, so there
// is no pause while the voice reloads. It does nothing for other engines.
func (a *App) PrefetchSpeech(text string) {
	if text == "" || lookupSpeechEngine() != "piper" {
		return
	}
	setup, ok := findPiper()
	if !ok {
		return
	}
	a.speechMu.Lock()
	if a.speechNext != nil && a.speechNext.text == text {
		a.speechMu.Unlock()
		return
	}
	pf := &prefetched{text: text, done: make(chan struct{})}
	a.speechNext = pf
	a.speechMu.Unlock()

	go func() {
		pf.data, pf.err = synthesizePiper(setup, text)
		close(pf.done)
	}()
}

// takePrefetched returns the prefetched audio for text, if there is any.
func (a *App) takePrefetched(text string) *prefetched {
	a.speechMu.Lock()
	defer a.speechMu.Unlock()
	pf := a.speechNext
	if pf == nil || pf.text != text {
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
func (a *App) Speak(text string) error {
	engine := a.SpeechEngine()
	if engine == "" {
		return fmt.Errorf("no speech engine installed")
	}
	if text == "" {
		return nil
	}

	var cmds []*exec.Cmd
	if engine == "piper" {
		setup, _ := findPiper()
		playerArgs, _ := setup.player()
		play := exec.Command(playerArgs[0], playerArgs[1:]...)

		if pf := a.takePrefetched(text); pf != nil {
			<-pf.done
			if pf.err == nil && len(pf.data) > 0 {
				// Already synthesized while the previous chunk was playing
				play.Stdin = bytes.NewReader(pf.data)
				cmds = []*exec.Cmd{play}
			}
		}
		if cmds == nil {
			synth := exec.Command(setup.bin, "--model", setup.model, "--config", setup.config, "--output-raw")
			synth.Stdin = strings.NewReader(text)
			pipe, err := synth.StdoutPipe()
			if err != nil {
				return err
			}
			play.Stdin = pipe
			cmds = []*exec.Cmd{synth, play}
		}
	} else {
		argv := speechCommand(engine, text)
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
