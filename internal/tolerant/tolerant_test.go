package tolerant

import (
	"bytes"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
)

type rec struct {
	Name  string    `datastore:"name"`
	Score float64   `datastore:"score"`
	N     int       `datastore:"n"`
	Blob  []byte    `datastore:"blob,noindex"`
	Tags  []string  `datastore:"tags"`
	At    time.Time `datastore:"at"`

	unknown Unknown
}

func (r *rec) Load(ps []datastore.Property) error  { return Load("Rec", r, ps, &r.unknown) }
func (r *rec) Save() ([]datastore.Property, error) { return Save(r, &r.unknown) }

var _ datastore.PropertyLoadSaver = (*rec)(nil)

func prop(name string, v any) datastore.Property { return datastore.Property{Name: name, Value: v} }

func find(ps []datastore.Property, name string) (datastore.Property, int) {
	var out datastore.Property
	n := 0
	for _, p := range ps {
		if p.Name == name {
			out = p
			n++
		}
	}
	return out, n
}

func count(kind, prop, problem string) int64 {
	for _, c := range Counts() {
		if c.Kind == kind && c.Property == prop && c.Problem == problem {
			return c.N
		}
	}
	return 0
}

// The datastore package's reason for a property without a field is what Load
// tells unknown properties by; a library update that rewords it must fail here.
func TestNoSuchFieldReasonPinned(t *testing.T) {
	var plain struct{ A string }
	err := datastore.LoadStruct(&plain, []datastore.Property{prop("B", "x")})
	fm, ok := err.(*datastore.ErrFieldMismatch)
	if !ok || fm.Reason != noSuchField {
		t.Fatalf("LoadStruct with an unknown property: %v, want reason %q", err, noSuchField)
	}
}

func TestLoadExtraMissingMistyped(t *testing.T) {
	reset()
	r := rec{Name: "stale", unknown: Unknown{extra: []datastore.Property{prop("old", 1)}}}
	err := r.Load([]datastore.Property{
		prop("name", "Ana"),
		prop("future", "x"),                // extra
		prop("score", int64(3)),            // int where a float is stored
		prop("n", 4.0),                     // integral float into an int
		prop("blob", "bytes"),              // string into []byte
		prop("tags", []any{"a", int64(1)}), // a list with a bad element
		prop("at", "yesterday"),            // beyond conversion
		// "n" present, nothing for missing fields: they stay zero
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.Name != "Ana" || r.Score != 3 || r.N != 4 || string(r.Blob) != "bytes" {
		t.Errorf("loaded %+v", r)
	}
	if r.Tags != nil {
		t.Errorf("Tags = %v: a list that failed halfway must leave the field zero", r.Tags)
	}
	if !r.At.IsZero() {
		t.Errorf("At = %v, want zero", r.At)
	}
	if r.unknown.Len() != 3 { // future, tags, at; the stale "old" is gone
		t.Errorf("unknown holds %d properties, want 3", r.unknown.Len())
	}
	for _, c := range []struct{ prop, problem string }{
		{"future", ProblemUnknown}, {"score", ProblemConverted}, {"n", ProblemConverted},
		{"blob", ProblemConverted}, {"tags", ProblemSkipped}, {"at", ProblemSkipped},
	} {
		if n := count("Rec", c.prop, c.problem); n != 1 {
			t.Errorf("count(%s, %s) = %d, want 1", c.prop, c.problem, n)
		}
	}
}

func TestMissingFieldsAreZeroed(t *testing.T) {
	r := rec{Name: "from before", N: 7}
	if err := r.Load([]datastore.Property{prop("score", 1.5)}); err != nil {
		t.Fatal(err)
	}
	if r.Name != "" || r.N != 0 || r.Score != 1.5 {
		t.Errorf("loaded %+v, want only score", r)
	}
}

// With nothing unknown, Save writes exactly the struct's schema.
func TestSaveIsStrict(t *testing.T) {
	r := rec{Name: "Ana", Score: 2, Tags: []string{"x"}, Blob: []byte{1}}
	got, err := r.Save()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := datastore.SaveStruct(&r)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Save = %v\nwant %v", got, want)
	}
}

func TestUnknownRoundTrip(t *testing.T) {
	in := []datastore.Property{
		prop("name", "Ana"),
		{Name: "srs_trace", Value: []byte{1, 2, 3}, NoIndex: true},
		prop("name2", []any{"a", "b"}),
	}
	var r rec
	if err := r.Load(in); err != nil {
		t.Fatal(err)
	}
	r.Name = "Bea"
	out, err := r.Save()
	if err != nil {
		t.Fatal(err)
	}
	if p, n := find(out, "srs_trace"); n != 1 || !p.NoIndex || !bytes.Equal(p.Value.([]byte), []byte{1, 2, 3}) {
		t.Errorf("srs_trace after the round trip: %+v (%d times), want it unchanged and unindexed", p, n)
	}
	if p, n := find(out, "name2"); n != 1 || !reflect.DeepEqual(p.Value, []any{"a", "b"}) {
		t.Errorf("name2 after the round trip: %+v (%d times)", p, n)
	}
	if p, _ := find(out, "name"); p.Value != "Bea" {
		t.Errorf("name = %v, want the new value", p.Value)
	}
	// The round trip loads again without loss.
	var again rec
	if err := again.Load(out); err != nil || again.Name != "Bea" || again.unknown.Len() != 2 {
		t.Errorf("reload: %v, %+v", err, again)
	}
}

// A known property that could not be read is written back while its field is
// still zero; once code sets the field, ours is written.
func TestUnreadableKeptUntilSet(t *testing.T) {
	var r rec
	if err := r.Load([]datastore.Property{prop("at", "yesterday")}); err != nil {
		t.Fatal(err)
	}
	out, _ := r.Save()
	if p, n := find(out, "at"); n != 1 || p.Value != "yesterday" {
		t.Errorf("at, field untouched: %+v (%d times), want the stored value back", p, n)
	}
	now := time.Now().Truncate(time.Microsecond)
	r.At = now
	out, _ = r.Save()
	if p, n := find(out, "at"); n != 1 || !p.Value.(time.Time).Equal(now) {
		t.Errorf("at, field set: %+v (%d times), want the new time", p, n)
	}
}

// The log names kind, property and problem once, never a value.
func TestLogOnceWithoutValues(t *testing.T) {
	reset()
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)
	for i := 0; i < 3; i++ {
		var r rec
		_ = r.Load([]datastore.Property{prop("secret_field", "s3cr3t-value"), prop("n", "also-secret")})
	}
	got := buf.String()
	if strings.Contains(got, "s3cr3t") || strings.Contains(got, "also-secret") {
		t.Errorf("a value reached the log:\n%s", got)
	}
	if strings.Count(got, "secret_field") != 1 || strings.Count(got, `"n"`) != 1 {
		t.Errorf("want each mismatch logged once:\n%s", got)
	}
	if n := count("Rec", "secret_field", ProblemUnknown); n != 3 {
		t.Errorf("counted %d, want 3", n)
	}
}
