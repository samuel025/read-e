package main

import (
	"reflect"
	"testing"
)

func TestSpeechCommand(t *testing.T) {
	cases := []struct {
		engine string
		want   []string
	}{
		{"spd-say", []string{"spd-say", "-w", "--", "-dash start"}},
		{"espeak-ng", []string{"espeak-ng", "-s", "165", "--", "-dash start"}},
	}
	for _, c := range cases {
		if got := speechCommand(c.engine, "-dash start"); !reflect.DeepEqual(got, c.want) {
			t.Errorf("speechCommand(%q) = %v, want %v", c.engine, got, c.want)
		}
	}
}
