package ripetoj

import (
	"reflect"
	"testing"
	"time"
)

var day0 = time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC)

// A record is 7 bytes and comes back as it went in; the day, count and
// grade fit their bits.
func TestPacking(t *testing.T) {
	rs := []Record{
		{Hash: 0xDEADBEEF, Day: 0, Count: 1, Grade: 1},
		{Hash: 1, Day: 0xFFFF, Count: MaxCount, Grade: 4},
		{Hash: Hash("mc-a1-001"), Day: DayOf(day0), Count: 7, Grade: 3},
	}
	b := Encode(rs)
	if len(b) != 3*RecordSize || RecordSize != 7 {
		t.Fatalf("%d bytes for 3 records", len(b))
	}
	if got := Decode(b); !reflect.DeepEqual(got, rs) {
		t.Errorf("round trip: %+v", got)
	}
	if Len(b) != 3 || Len(nil) != 0 || Encode(nil) != nil {
		t.Error("Len / empty")
	}
	if got := Decode(append(b, 1, 2, 3)); len(got) != 3 {
		t.Error("a partial record is not dropped")
	}
	if DayOf(time.Date(2020, 1, 1, 23, 0, 0, 0, time.UTC)) != 0 || DayOf(time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)) != 0 {
		t.Error("day 0")
	}
	if UnixDay(0) != 18262 {
		t.Errorf("UnixDay(0) = %d", UnixDay(0))
	}
}

// Note: one record per item, never one for an item not answered; the count
// grows to its cap, the day and grade follow the last answer; a bad grade
// or no slug change nothing.
func TestNote(t *testing.T) {
	var b []byte
	b = Note(b, "a", 3, day0)
	b = Note(b, "b", 1, day0)
	b = Note(b, "a", 2, day0.Add(48*time.Hour))
	if Len(b) != 2 {
		t.Fatalf("%d records for two items", Len(b))
	}
	rs := Decode(b)
	if rs[0] != (Record{Hash: Hash("a"), Day: DayOf(day0) + 2, Count: 2, Grade: 2}) {
		t.Errorf("a: %+v", rs[0])
	}
	if rs[1] != (Record{Hash: Hash("b"), Day: DayOf(day0), Count: 1, Grade: 1}) {
		t.Errorf("b: %+v", rs[1])
	}
	before := append([]byte(nil), b...)
	if !reflect.DeepEqual(Note(b, "a", 0, day0), before) || !reflect.DeepEqual(Note(b, "a", 5, day0), before) || !reflect.DeepEqual(Note(b, "", 3, day0), before) {
		t.Error("a bad grade or empty slug changed the trace")
	}
	if !reflect.DeepEqual(b, before) {
		t.Error("Note changed its input")
	}
	for i := 0; i < 100; i++ {
		b = Note(b, "b", 4, day0)
	}
	if r := Decode(b)[1]; r.Count != MaxCount || r.Grade != 4 {
		t.Errorf("capped: %+v", r)
	}
	// The size stays 7 bytes an item seen: 1000 items, 7000 bytes.
	var big []byte
	for i := 0; i < 1000; i++ {
		big = Note(big, "item-"+string(rune('a'+i%26))+string(rune('a'+i/26)), 3, day0)
	}
	if len(big) != 7000 {
		t.Errorf("1000 items take %d bytes", len(big))
	}
}

// Merge: per item the later day and its grade, the counts added (capped).
func TestMerge(t *testing.T) {
	a := Note(Note(nil, "x", 3, day0), "y", 1, day0)
	b := Note(Note(nil, "x", 1, day0.Add(24*time.Hour)), "z", 4, day0)
	m := Decode(Merge(a, b))
	if len(m) != 3 {
		t.Fatalf("%d records", len(m))
	}
	if m[0] != (Record{Hash: Hash("x"), Day: DayOf(day0) + 1, Count: 2, Grade: 1}) {
		t.Errorf("x: %+v", m[0])
	}
	if Merge(nil, nil) != nil {
		t.Error("two empty traces")
	}
}

// Resolve names the records by the course's slugs, leaving out items gone.
func TestResolve(t *testing.T) {
	b := Note(Note(Note(nil, "gone", 3, day0), "b", 4, day0), "a", 2, day0)
	got := Resolve(b, []string{"a", "b", "c"})
	want := []Entry{
		{Slug: "a", Day: UnixDay(DayOf(day0)), Count: 1, Grade: 2},
		{Slug: "b", Day: UnixDay(DayOf(day0)), Count: 1, Grade: 4},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve: %+v", got)
	}
	if UnixDay(DayOf(day0)) != int(day0.Unix()/86400) {
		t.Error("the unix day is not the answer's")
	}
}

// Shuffle is angla.org's fixedShuffle (values from its reguloj.js), a
// permutation, the same for the same seed, and fair: over many items the
// right answer (authored first) lands on every place about equally often.
func TestShuffle(t *testing.T) {
	for _, c := range []struct {
		n    int
		seed string
		want []int
	}{
		{4, "mc-a1-001", []int{2, 3, 0, 1}},
		{4, "zagr-l03-elekto-2", []int{0, 2, 3, 1}},
		{3, "ĉu-ŝi", []int{0, 1, 2}},
		{5, "x", []int{2, 4, 1, 3, 0}},
	} {
		if got := Shuffle(c.n, c.seed); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Shuffle(%d, %q) = %v, angla.org gives %v", c.n, c.seed, got, c.want)
		}
	}
	if len(Shuffle(0, "x")) != 0 || !reflect.DeepEqual(Shuffle(1, "x"), []int{0}) {
		t.Error("0 or 1 choices")
	}
	const items = 8000
	place := make([]int, 4)
	for i := 0; i < items; i++ {
		p := Shuffle(4, "item-"+time.Duration(i).String())
		seen := map[int]bool{}
		for k, j := range p {
			seen[j] = true
			if j == 0 {
				place[k]++
			}
		}
		if len(seen) != 4 {
			t.Fatalf("not a permutation: %v", p)
		}
	}
	for k, n := range place {
		if n < items/4*9/10 || n > items/4*11/10 {
			t.Errorf("the answer lands on place %d %d times of %d", k+1, n, items)
		}
	}
}
