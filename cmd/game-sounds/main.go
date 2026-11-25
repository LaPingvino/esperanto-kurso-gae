// Command game-sounds writes the game's two sounds, game/sounds/snd_right.wav
// and snd_wrong.wav: the site's own answer sounds, which static/eo.js makes
// with Web Audio (no files), rendered the same way — an oscillator at a set
// gain falling exponentially to 0.001 over its length:
//
//	right: 660 Hz sine, 0.12 s, gain 0.12; 100 ms later 880 Hz sine, 0.15 s, gain 0.10
//	wrong: 300 Hz sawtooth, 0.2 s, gain 0.10
//
// 16-bit mono WAV at 44100 Hz. Run it again after changing eo.js's sounds:
//
//	go run ./cmd/game-sounds [-out game/sounds]
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"log"
	"math"
	"os"
	"path/filepath"
)

const rate = 44100

// beep is eo.js's beep(freq, duration, type, vol), starting at `at` seconds.
type beep struct {
	at, freq, dur, vol float64
	saw                bool
}

// sounds are eo.js's playCorrect and playIncorrect.
var sounds = map[string][]beep{
	"snd_right": {{0, 660, 0.12, 0.12, false}, {0.1, 880, 0.15, 0.10, false}},
	"snd_wrong": {{0, 300, 0.2, 0.10, true}},
}

// wave is the oscillator at phase t (seconds): Web Audio's sine, or its
// sawtooth (band-limited: the harmonics under half the sample rate, rising
// from -1 to 1 as Web Audio's does).
func wave(b beep, t float64) float64 {
	if !b.saw {
		return math.Sin(2 * math.Pi * b.freq * t)
	}
	s := 0.0
	for k := 1; float64(k)*b.freq < rate/2; k++ {
		sign := 1.0
		if k%2 == 0 {
			sign = -1
		}
		s += sign * math.Sin(2*math.Pi*float64(k)*b.freq*t) / float64(k)
	}
	return 2 / math.Pi * s
}

// render mixes the beeps: each at its gain, ramped exponentially to 0.001 at
// its end (exponentialRampToValueAtTime), then stopped.
func render(bs []beep) []int16 {
	end := 0.0
	for _, b := range bs {
		end = max(end, b.at+b.dur)
	}
	out := make([]int16, int(end*rate)+1)
	for i := range out {
		t := float64(i) / rate
		v := 0.0
		for _, b := range bs {
			if t < b.at || t >= b.at+b.dur {
				continue
			}
			u := t - b.at
			g := b.vol * math.Pow(0.001/b.vol, u/b.dur)
			v += g * wave(b, u)
		}
		out[i] = int16(max(-1, min(1, v)) * 32767)
	}
	return out
}

func wav(samples []int16) []byte {
	var b bytes.Buffer
	w := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	n := uint32(len(samples) * 2)
	b.WriteString("RIFF")
	w(36 + n)
	b.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1)) // PCM
	w(uint16(1)) // mono
	w(uint32(rate))
	w(uint32(rate * 2))
	w(uint16(2))
	w(uint16(16))
	b.WriteString("data")
	w(n)
	w(samples)
	return b.Bytes()
}

func main() {
	out := flag.String("out", "game/sounds", "the game's sounds folder")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	for name, bs := range sounds {
		if err := os.WriteFile(filepath.Join(*out, name+".wav"), wav(render(bs)), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
