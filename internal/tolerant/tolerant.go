// Package tolerant loads Datastore entities leniently and saves them strictly
// ("accept freely, produce strictly"), so a version of the site can read
// entities written by a newer or older version of itself: a rollback after a
// deploy that added a property must not break loading.
//
// Every entity struct gets a pointer-receiver Load and Save that call Load and
// Save here (datastore.PropertyLoadSaver), with its kind and an Unknown field:
//
//	func (e *fooEntity) Load(ps []datastore.Property) error { return tolerant.Load(fooKind, e, ps, &e.unknown) }
//	func (e *fooEntity) Save() ([]datastore.Property, error) { return tolerant.Save(e, &e.unknown) }
//
// Loading never fails on the schema. Per property:
//   - known and of the right type: loaded as usual;
//   - unknown (no struct field of that name): kept in Unknown and written back
//     unchanged on Save, with its index flag, so an older version does not
//     erase what a newer one stored (the Ripetoj trace, srs_trace, say);
//   - known but of another type: converted where no information is lost
//     (int↔float when integral, string↔[]byte, a one-element list to a
//     single value); otherwise skipped, and the field keeps its zero value;
//   - missing: the field keeps its zero value (Load zeroes the struct first).
//
// Saving writes exactly the struct's own schema (datastore.SaveStruct), plus the
// Unknown properties under names the struct does not use. One exception, for
// the same reason as Unknown: a known property that could not be read (its
// type is beyond conversion) is written back as it was, as long as the field
// still holds its zero value on Save; once code sets the field, ours wins.
//
// The preserved properties may be stale relative to the fields this version
// changed (a trace next to progress it did not see). That is the safe side:
// the version that knows them reads and corrects them, and nothing is lost.
// To retire a property for good, delete it from the stored entities; dropping
// the struct field alone leaves it preserved (and counted as unknown).
//
// Every tolerated mismatch is counted per kind, property, problem and app
// version (Counts) and logged once per that tuple. Names only: values never
// reach the log.
package tolerant

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"reflect"
	"sort"
	"sync"
	"unicode/utf8"

	"cloud.google.com/go/datastore"
)

// Unknown holds the properties of a loaded entity that its struct does not
// represent: the unknown ones and the known ones that could not be read. The
// zero value is empty. Copying an Unknown shares its (read-only) properties.
type Unknown struct {
	extra      []datastore.Property // no struct field: written back as is
	unreadable []datastore.Property // struct field, unconvertible type: written back while the field is zero
}

// Len is the number of properties held.
func (u *Unknown) Len() int {
	if u == nil {
		return 0
	}
	return len(u.extra) + len(u.unreadable)
}

// Problems, as counted and logged.
const (
	ProblemUnknown   = "unknown"   // no such field: kept, written back on save
	ProblemConverted = "converted" // another type, converted without loss
	ProblemSkipped   = "skipped"   // another type, beyond conversion: field left zero
)

// noSuchField is the reason the datastore package gives for a property its
// struct has no field for (load.go); tolerant_test pins it.
const noSuchField = "no such struct field"

// Load loads ps into dst, a pointer to a struct, as described in the package
// comment, and records in u what dst cannot hold. It fails only when dst is
// not a struct pointer the datastore package accepts.
func Load(kind string, dst any, ps []datastore.Property, u *Unknown) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("tolerant: Load needs a struct pointer, got %T", dst)
	}
	*u = Unknown{}
	sv := rv.Elem()
	sv.Set(reflect.Zero(sv.Type()))
	// Each property is first tried alone on a scratch value, so a property
	// that fails halfway (a list with one bad element) never leaves a field
	// half set; the accepted ones are then loaded into dst together.
	scratch := reflect.New(sv.Type())
	try := func(p datastore.Property) (string, error) {
		scratch.Elem().Set(reflect.Zero(sv.Type()))
		err := datastore.LoadStruct(scratch.Interface(), []datastore.Property{p})
		var fm *datastore.ErrFieldMismatch
		switch {
		case err == nil:
			return "", nil
		case errors.As(err, &fm):
			return fm.Reason, nil
		default:
			return "", err
		}
	}
	accepted := make([]datastore.Property, 0, len(ps))
	for _, p := range ps {
		reason, err := try(p)
		if err != nil {
			return fmt.Errorf("tolerant: %s: %w", kind, err)
		}
		if reason == "" {
			accepted = append(accepted, p)
			continue
		}
		if reason == noSuchField {
			u.extra = append(u.extra, p)
			note(kind, p.Name, ProblemUnknown)
			continue
		}
		converted := false
		for _, v := range conversions(p.Value) {
			q := p
			q.Value = v
			if r, err := try(q); err == nil && r == "" {
				accepted = append(accepted, q)
				converted = true
				break
			}
		}
		if converted {
			note(kind, p.Name, ProblemConverted)
		} else {
			u.unreadable = append(u.unreadable, p)
			note(kind, p.Name, ProblemSkipped)
		}
	}
	if err := datastore.LoadStruct(dst, accepted); err != nil {
		// Every property was accepted alone; together they still should be.
		// Should that ever fail, keep what loaded rather than fail the entity.
		var fm *datastore.ErrFieldMismatch
		if !errors.As(err, &fm) {
			return fmt.Errorf("tolerant: %s: %w", kind, err)
		}
		note(kind, fm.FieldName, ProblemSkipped)
	}
	return nil
}

// Save returns src's properties (datastore.SaveStruct) followed by the
// properties held in u under names src does not write.
func Save(src any, u *Unknown) ([]datastore.Property, error) {
	ps, err := datastore.SaveStruct(src)
	if err != nil || u.Len() == 0 {
		return ps, err
	}
	at := make(map[string]int, len(ps))
	for i, p := range ps {
		at[p.Name] = i
	}
	for _, p := range u.unreadable {
		i, ok := at[p.Name]
		switch {
		case !ok:
			at[p.Name] = len(ps)
			ps = append(ps, p)
		case isZero(ps[i].Value):
			ps[i] = p
		}
	}
	for _, p := range u.extra {
		if _, ok := at[p.Name]; !ok {
			at[p.Name] = len(ps)
			ps = append(ps, p)
		}
	}
	return ps, nil
}

func isZero(v any) bool {
	if v == nil {
		return true
	}
	return reflect.ValueOf(v).IsZero()
}

// conversions lists the values v can be read as without losing information,
// for a struct field of another type; Load tries them in order.
func conversions(v any) []any {
	var out []any
	switch x := v.(type) {
	case int64:
		out = append(out, float64(x))
	case float64:
		if x == math.Trunc(x) && x >= -(1<<63) && x < 1<<63 {
			out = append(out, int64(x))
		}
	case string:
		out = append(out, []byte(x))
	case []byte:
		if utf8.Valid(x) {
			out = append(out, string(x))
		}
	case []any:
		if len(x) == 1 {
			out = append(out, x[0])
			out = append(out, conversions(x[0])...)
		}
		// Element by element: []int64 into []float64, say.
		if len(x) > 0 {
			conv := make([]any, len(x))
			for i, e := range x {
				c := conversions(e)
				if len(c) == 0 {
					return out
				}
				conv[i] = c[0]
			}
			out = append(out, conv)
		}
	}
	return out
}

// Count is one tolerated mismatch and how often it was seen by this instance.
type Count struct {
	Kind, Property, Problem, Version string
	N                                int64
}

type countKey struct{ kind, prop, problem, version string }

var (
	mu     sync.Mutex
	counts = map[countKey]int64{}
)

// version is the App Engine version serving this instance.
var version = func() string {
	if v := os.Getenv("GAE_VERSION"); v != "" {
		return v
	}
	return "local"
}()

// note counts a mismatch and logs it the first time this instance sees it.
// Kind and property names only: never a value.
func note(kind, prop, problem string) {
	k := countKey{kind, prop, problem, version}
	mu.Lock()
	counts[k]++
	first := counts[k] == 1
	mu.Unlock()
	if first {
		log.Printf("tolerant: datastore kind %s property %q: %s (version %s; logged once per instance)", kind, prop, problem, version)
	}
}

// Counts returns this instance's tolerated mismatches since it started,
// sorted by kind, property and problem. Each instance counts its own.
func Counts() []Count {
	mu.Lock()
	out := make([]Count, 0, len(counts))
	for k, n := range counts {
		out = append(out, Count{k.kind, k.prop, k.problem, k.version, n})
	}
	mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Property != b.Property {
			return a.Property < b.Property
		}
		if a.Problem != b.Problem {
			return a.Problem < b.Problem
		}
		return a.Version < b.Version
	})
	return out
}

// reset clears the counts (tests).
func reset() {
	mu.Lock()
	counts = map[countKey]int64{}
	mu.Unlock()
}
