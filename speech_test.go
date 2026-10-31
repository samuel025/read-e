package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"reflect"
	"testing"
	"testing/iotest"
)

func TestSpeechCommand(t *testing.T) {
	cases := []struct {
		name   string
		engine string
		speed  float64
		volume int
		want   []string
	}{
		{"spd-say defaults", "spd-say", 0, 0, []string{"spd-say", "-w", "-r", "0", "-i", "100", "--", "-dash start"}},
		{"spd-say fast and quiet", "spd-say", 1.5, 30, []string{"spd-say", "-w", "-r", "50", "-i", "-40", "--", "-dash start"}},
		{"spd-say slow", "spd-say", 0.5, 100, []string{"spd-say", "-w", "-r", "-50", "-i", "100", "--", "-dash start"}},
		{"espeak defaults", "espeak-ng", 1, 100, []string{"espeak-ng", "-s", "165", "-a", "200", "--", "-dash start"}},
		{"espeak scaled", "espeak-ng", 2, 50, []string{"espeak-ng", "-s", "330", "-a", "100", "--", "-dash start"}},
	}
	for _, c := range cases {
		if got := speechCommand(c.engine, "-dash start", c.speed, c.volume); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	if normalizeSpeed(0) != 1 || normalizeSpeed(9) != maxSpeechSpeed || normalizeSpeed(0.1) != minSpeechSpeed {
		t.Error("speed normalisation")
	}
	if normalizeVolume(0) != 100 || normalizeVolume(500) != maxSpeechVolume || normalizeVolume(1) != minSpeechVolume {
		t.Error("volume normalisation")
	}
}

func pcm(samples ...int16) []byte {
	b := make([]byte, 2*len(samples))
	for i, s := range samples {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(s))
	}
	return b
}

func TestApplyGain(t *testing.T) {
	got := pcm(1000, -1000, 30000, -30000)
	applyGain(got, 0.5)
	if want := pcm(500, -500, 15000, -15000); !bytes.Equal(got, want) {
		t.Errorf("half gain: % x, want % x", got, want)
	}
	got = pcm(1000, 30000, -30000)
	applyGain(got, 1.5)
	if want := pcm(1500, 32767, -32768); !bytes.Equal(got, want) {
		t.Errorf("boost must clamp, not wrap: % x, want % x", got, want)
	}
	same := pcm(123, -456)
	applyGain(same, 1)
	if !bytes.Equal(same, pcm(123, -456)) {
		t.Error("gain 1 must be a no-op")
	}
}

type sink struct{ bytes.Buffer }

func (*sink) Close() error { return nil }

func TestCopyWithGainHandlesSplitSamples(t *testing.T) {
	src := pcm(100, -200, 300, -400, 500, -600, 700)
	want := append([]byte(nil), src...)
	applyGain(want, 0.5)

	var out sink
	// One byte per read splits every 16-bit sample across reads
	copyWithGain(&out, iotest.OneByteReader(bytes.NewReader(src)), 0.5)
	if !bytes.Equal(out.Bytes(), want) {
		t.Errorf("streamed output differs:\n got % x\nwant % x", out.Bytes(), want)
	}
	_ = io.EOF
}

func TestPiperTiming(t *testing.T) {
	if scale, silence := piperTiming(1); scale != 1 || silence != 0.2 {
		t.Errorf("speed 1 must be neutral, got scale %v silence %v", scale, silence)
	}
	prev := 100.0
	for _, sp := range []float64{0.6, 0.8, 1, 1.25, 1.5, 2, 2.5} {
		scale, _ := piperTiming(sp)
		if scale >= prev {
			t.Errorf("length scale must fall as speed rises (speed %v: %v >= %v)", sp, scale, prev)
		}
		if scale < 0.3 {
			t.Errorf("length scale below the floor at speed %v: %v", sp, scale)
		}
		prev = scale
	}
}
