package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// The embedded web view (WebKitGTK) often ships without the browser speech API,
// so read aloud drives a system speech program instead.
//
// Engines in order of preference:
//   - spd-say  (speech-dispatcher; honours the user's configured voice/language)
//   - espeak-ng, espeak (direct)
var speechEngines = []string{"spd-say", "espeak-ng", "espeak"}

// speechCommand builds the command line that speaks text with the given engine.
// The text follows "--" so a chunk beginning with "-" is not read as an option.
func speechCommand(engine, text string) []string {
	switch engine {
	case "spd-say":
		return []string{engine, "-w", "--", text} // -w: return when the message is done
	default:
		return []string{engine, "-s", "165", "--", text}
	}
}

func lookupSpeechEngine() string {
	for _, name := range speechEngines {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

// SpeechEngine reports which speech program read aloud will use ("" if none).
func (a *App) SpeechEngine() string {
	a.speechOnce.Do(func() { a.speechEngine = lookupSpeechEngine() })
	return a.speechEngine
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

	argv := speechCommand(engine, text)
	cmd := exec.Command(argv[0], argv[1:]...)

	a.speechMu.Lock()
	a.speechCmd = cmd
	gen := a.speechGen
	a.speechMu.Unlock()

	err := cmd.Run()

	a.speechMu.Lock()
	interrupted := a.speechGen != gen
	if a.speechCmd == cmd {
		a.speechCmd = nil
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
	cmd := a.speechCmd
	a.speechGen++ // marks any Speak in flight as interrupted
	a.speechMu.Unlock()

	if a.SpeechEngine() == "spd-say" {
		// Killing the client is not enough: the speech-dispatcher daemon keeps speaking
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = exec.CommandContext(ctx, "spd-say", "-C").Run()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
