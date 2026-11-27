// Package ripetoj is the server's side of spaced repetition (Ripetoj). The
// scheduling itself runs in the browser (static/fsrs.js, static/ripetoj.js)
// with its state in localStorage; the server keeps only a rough trace per
// learner, enough for a browser with no state of its own to rebuild a
// reasonable one (static/ripetoj.js rebuild()).
//
// The trace ("spuro") is a byte string of fixed-width records, one per item
// the learner has answered (items never seen take no room):
//
//	bytes 0–3  FNV-1a 32 of the item's slug (big-endian)
//	bytes 4–5  the day of the last answer, days since 2020-01-01 UTC (big-endian)
//	byte  6    the number of answers (capped at 63) << 2 | (last grade − 1)
//
// So 7 bytes per item: a learner with 1000 items seen carries 7 kB on their
// User entity, written in the transaction that already records the streak.
// Two slugs may share a hash (for 5000 items, odds about 0.3 %); they then
// share a record, which only blurs that one rebuild.
//
// Grades are FSRS's: 1 Again (ne sciis / wrong), 2 Hard (malfacile),
// 3 Good (bone / right), 4 Easy (facile).
package ripetoj

import (
	"encoding/binary"
	"hash/fnv"
	"sort"
	"time"
	"unicode/utf16"
)

// RecordSize is the bytes per item in a trace.
const RecordSize = 7

// MaxCount is where the answer count stops growing.
const MaxCount = 63

var epoch = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// Record is one item's trace.
type Record struct {
	Hash  uint32
	Day   int // days since 2020-01-01 UTC
	Count int // answers, 1–63
	Grade int // the last one, 1–4
}

// Hash is the slug's FNV-1a 32.
func Hash(slug string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(slug))
	return h.Sum32()
}

// DayOf is t's day number in a trace.
func DayOf(t time.Time) int {
	d := int(t.UTC().Sub(epoch).Hours() / 24)
	if d < 0 {
		return 0
	}
	if d > 0xFFFF {
		return 0xFFFF
	}
	return d
}

// UnixDay is a trace day as days since 1970-01-01 (what the browser uses).
func UnixDay(day int) int { return day + int(epoch.Unix()/86400) }

func put(b []byte, r Record) {
	binary.BigEndian.PutUint32(b[0:4], r.Hash)
	binary.BigEndian.PutUint16(b[4:6], uint16(r.Day))
	b[6] = byte(r.Count<<2 | (r.Grade-1)&3)
}

func get(b []byte) Record {
	return Record{
		Hash:  binary.BigEndian.Uint32(b[0:4]),
		Day:   int(binary.BigEndian.Uint16(b[4:6])),
		Count: int(b[6] >> 2),
		Grade: int(b[6]&3) + 1,
	}
}

// Decode is a trace's records (a trailing partial record is dropped).
func Decode(b []byte) []Record {
	out := make([]Record, 0, len(b)/RecordSize)
	for i := 0; i+RecordSize <= len(b); i += RecordSize {
		out = append(out, get(b[i:i+RecordSize]))
	}
	return out
}

// Encode packs records (in the order given).
func Encode(rs []Record) []byte {
	if len(rs) == 0 {
		return nil
	}
	b := make([]byte, len(rs)*RecordSize)
	for i, r := range rs {
		put(b[i*RecordSize:], r)
	}
	return b
}

// Len is the number of items in a trace.
func Len(b []byte) int { return len(b) / RecordSize }

// Note adds an answer to the trace: the item's day and grade, its count up
// by one. A grade outside 1–4 changes nothing.
func Note(b []byte, slug string, grade int, now time.Time) []byte {
	if grade < 1 || grade > 4 || slug == "" {
		return b
	}
	h, day := Hash(slug), DayOf(now)
	for i := 0; i+RecordSize <= len(b); i += RecordSize {
		r := get(b[i : i+RecordSize])
		if r.Hash != h {
			continue
		}
		if r.Count < MaxCount {
			r.Count++
		}
		r.Day, r.Grade = day, grade
		out := append([]byte(nil), b[:len(b)-len(b)%RecordSize]...)
		put(out[i:], r)
		return out
	}
	out := make([]byte, len(b)-len(b)%RecordSize, len(b)+RecordSize)
	copy(out, b)
	rec := make([]byte, RecordSize)
	put(rec, Record{Hash: h, Day: day, Count: 1, Grade: grade})
	return append(out, rec...)
}

// Merge is two traces as one (two accounts merged): per item the later
// answer's day and grade, the counts added.
func Merge(a, b []byte) []byte {
	by := map[uint32]Record{}
	var order []uint32
	for _, r := range append(Decode(a), Decode(b)...) {
		old, ok := by[r.Hash]
		if !ok {
			by[r.Hash] = r
			order = append(order, r.Hash)
			continue
		}
		n := old.Count + r.Count
		if n > MaxCount {
			n = MaxCount
		}
		if r.Day > old.Day {
			old.Day, old.Grade = r.Day, r.Grade
		}
		old.Count = n
		by[r.Hash] = old
	}
	rs := make([]Record, len(order))
	for i, h := range order {
		rs[i] = by[h]
	}
	return Encode(rs)
}

// Entry is one item of a trace by slug, as the browser gets it.
type Entry struct {
	Slug  string
	Day   int // days since 1970-01-01
	Count int
	Grade int
}

// Resolve names a trace's records by the slugs given (the current content):
// records no slug matches (items gone) are left out. Sorted by slug.
func Resolve(b []byte, slugs []string) []Entry {
	recs := Decode(b)
	if len(recs) == 0 {
		return nil
	}
	by := make(map[uint32]Record, len(recs))
	for _, r := range recs {
		by[r.Hash] = r
	}
	var out []Entry
	for _, s := range slugs {
		if r, ok := by[Hash(s)]; ok {
			out = append(out, Entry{Slug: s, Day: UnixDay(r.Day), Count: r.Count, Grade: r.Grade})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// fnv16 is FNV-1a over UTF-16 code units, as the browser's hash (and
// angla.org's) computes it from a string.
func fnv16(s string) uint32 {
	h := uint32(2166136261)
	for _, c := range utf16.Encode([]rune(s)) {
		h ^= uint32(c)
		h *= 16777619
	}
	return h
}

// Shuffle is a fixed, fair permutation of n choices for the seed (an item's
// slug): Fisher–Yates, each step drawing from the high bits of a mixed hash.
// The authored order may come back. The same as angla.org's fixedShuffle
// (reguloj.js), so the two courses shuffle alike.
func Shuffle(n int, seed string) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	h := fnv16(seed)
	for j := n - 1; j > 0; j-- {
		h = (h ^ h>>16) * 0x85ebca6b
		h = (h ^ h>>13) * 0xc2b2ae35
		h = h ^ h>>16
		k := int(uint64(h) * uint64(j+1) >> 32)
		p[j], p[k] = p[k], p[j]
		h += 0x9e3779b9
	}
	return p
}

// The grades.
const (
	Again = 1
	Hard  = 2
	Good  = 3
	Easy  = 4
)
