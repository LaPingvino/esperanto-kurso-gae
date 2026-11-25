// Command revo-import builds internal/revo/revo.json.gz from a git clone of
// Reta Vortaro's sources (github.com/revuloj/revo-fonto, GPL-2.0) and the
// character entities of its DTD (github.com/revuloj/voko-grundo, GPL-2.0).
//
//	git clone --depth 1 https://github.com/revuloj/revo-fonto.git  $SRC/revo-fonto
//	git clone --depth 1 https://github.com/revuloj/voko-grundo.git $SRC/voko-grundo
//	go run ./cmd/revo-import -fonto $SRC/revo-fonto -grundo $SRC/voko-grundo
//
// Per derivation (drv) it keeps the marker (for deep links), the headwords,
// the first Esperanto definition and the translations, grouped per sense.
// The site never fetches from reta-vortaro.de at run time: lookups read the
// embedded file.
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/revo"
)

func main() {
	fonto := flag.String("fonto", "", "clone of revuloj/revo-fonto")
	grundo := flag.String("grundo", "", "clone of revuloj/voko-grundo (for dtd/vokosgn.dtd)")
	out := flag.String("o", "internal/revo/revo.json.gz", "output file")
	flag.Parse()
	if *fonto == "" || *grundo == "" {
		flag.Usage()
		os.Exit(2)
	}

	ent, err := loadEntities(filepath.Join(*grundo, "dtd"), "vokosgn.dtd", "vokomll.dtd", "vokourl.dtd")
	entities = ent
	if err != nil {
		log.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(*fonto, "revo", "*.xml"))
	sort.Strings(files)
	if len(files) == 0 {
		log.Fatalf("no articles in %s/revo", *fonto)
	}

	d := &revo.Data{
		Source: "Reta Vortaro (reta-vortaro.de), github.com/revuloj/revo-fonto, GPL-2.0",
		Roots:  map[string]int{},
	}
	if c, err := exec.Command("git", "-C", *fonto, "log", "-1", "--format=%H %cs").Output(); err == nil {
		if f := strings.Fields(string(c)); len(f) == 2 {
			d.Commit, d.Date = f[0], f[1]
		}
	}
	skipped := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		art, err := parseArticle(raw, ent)
		if err != nil {
			log.Printf("%s: %v (skipped)", filepath.Base(f), err)
			skipped++
			continue
		}
		first := len(d.Drv)
		for _, dv := range art.drvs {
			trd := map[string]string{}
			for lng, senses := range dv.trd {
				if s := revo.JoinSenses(senses); s != "" {
					trd[lng] = s
				}
			}
			e := revo.Entry{Mrk: dv.mrk, Heads: dv.heads, Def: dv.def, Trd: revo.EncodeTrd(trd)}
			if len(e.Heads) == 0 {
				continue
			}
			d.Drv = append(d.Drv, e)
		}
		if len(d.Drv) > first {
			for _, r := range art.roots {
				if _, ok := d.Roots[r]; !ok {
					d.Roots[r] = first
				}
			}
		}
	}

	if skipped > 20 {
		log.Fatalf("%d of %d articles did not parse: is -grundo the voko-grundo clone that goes with this revo-fonto?", skipped, len(files))
	}

	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	enc := json.NewEncoder(zw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(d); err != nil {
		log.Fatal(err)
	}
	zw.Close()
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("%d articles, %d derivations, %d roots → %s (%d bytes)", len(files), len(d.Drv), len(d.Roots), *out, buf.Len())
}

// loadEntities reads the <!ENTITY name "value"> lines of ReVo's DTD files
// (characters, abbreviations, URLs) into the decoder's entity map,
// resolving numeric references and references to other entities.
func loadEntities(dir string, names ...string) (map[string]string, error) {
	re := regexp.MustCompile(`<!ENTITY\s+([A-Za-z0-9_.-]+)\s+"([^"]*)"`)
	num := regexp.MustCompile(`&#(x[0-9a-fA-F]+|[0-9]+);`)
	named := regexp.MustCompile(`&([A-Za-z0-9_.-]+);`)
	rawEnt := map[string]string{}
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
			if m[1] == "amp" || m[1] == "lt" || m[1] == "gt" {
				continue // the XML decoder knows these
			}
			rawEnt[m[1]] = m[2]
		}
	}
	ent := map[string]string{}
	var resolve func(name string, depth int) string
	resolve = func(name string, depth int) string {
		if v, ok := ent[name]; ok {
			return v
		}
		v := num.ReplaceAllStringFunc(rawEnt[name], func(s string) string {
			s = strings.TrimSuffix(strings.TrimPrefix(s, "&#"), ";")
			var n uint64
			if strings.HasPrefix(s, "x") {
				n, _ = strconv.ParseUint(s[1:], 16, 32)
			} else {
				n, _ = strconv.ParseUint(s, 10, 32)
			}
			return string(rune(n))
		})
		if depth < 5 {
			v = named.ReplaceAllStringFunc(v, func(s string) string {
				k := s[1 : len(s)-1]
				if _, ok := rawEnt[k]; ok {
					return resolve(k, depth+1)
				}
				return s
			})
		}
		ent[name] = v
		return v
	}
	for name := range rawEnt {
		resolve(name, 0)
	}
	return ent, nil
}

type drv struct {
	mrk   string
	heads []string
	def   string
	trd   map[string][][]string // lng → senses → words
}

type article struct {
	roots []string
	drvs  []*drv
}

// node is a minimal element tree: enough to walk ReVo's markup.
type node struct {
	name string
	attr map[string]string
	kids []*node
	text string // for text nodes (name == "")
}

func parseArticle(raw []byte, ent map[string]string) (*article, error) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	dec.Entity = ent
	dec.Strict = true
	root := &node{name: "#doc"}
	stack := []*node{root}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		top := stack[len(stack)-1]
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{name: t.Name.Local, attr: map[string]string{}}
			for _, a := range t.Attr {
				n.attr[a.Name.Local] = a.Value
			}
			top.kids = append(top.kids, n)
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			top.kids = append(top.kids, &node{text: string(t)})
		}
	}
	art := find(root, "art")
	if art == nil {
		return nil, fmt.Errorf("no <art>")
	}
	a := &article{}
	kap := child(art, "kap")
	if kap == nil {
		return nil, fmt.Errorf("no article <kap>")
	}
	rad := find(kap, "rad")
	if rad == nil {
		return nil, fmt.Errorf("no <rad>")
	}
	radText := strings.TrimSpace(plain(rad))
	a.roots = append(a.roots, revo.Normalize(radText))
	// Variant roots: <var><kap><rad var="…">…</rad>…</kap></var>.
	walk(kap, func(n *node) bool {
		if n.name == "var" {
			if r := find(n, "rad"); r != nil {
				a.roots = append(a.roots, revo.Normalize(plain(r)))
			}
			return false
		}
		return true
	})

	var collect func(n *node)
	collect = func(n *node) {
		for _, k := range n.kids {
			switch k.name {
			case "drv":
				d := &drv{mrk: k.attr["mrk"], trd: map[string][][]string{}}
				if h := child(k, "kap"); h != nil {
					d.heads = heads(h, radText)
				}
				d.def = firstDef(k, radText)
				addSenses(k, d)
				a.drvs = append(a.drvs, d)
			case "subart":
				collect(k)
			}
		}
	}
	collect(art)
	return a, nil
}

// addSenses fills d.trd: one sense bucket per snc/subsnc that has
// translations, plus one for translations directly under the drv.
func addSenses(dn *node, d *drv) {
	var visit func(n *node, bucket int)
	order := 0
	type key struct {
		lng    string
		bucket int
	}
	fak := map[int]bool{}
	got := map[key][]string{}
	var keys []key
	add := func(lng string, bucket int, words []string) {
		if lng == "" || len(words) == 0 {
			return
		}
		k := key{lng, bucket}
		if _, ok := got[k]; !ok {
			keys = append(keys, k)
		}
		got[k] = append(got[k], words...)
	}
	visit = func(n *node, bucket int) {
		for _, k := range n.kids {
			switch k.name {
			case "snc", "subsnc":
				order++
				fak[order] = fak[bucket] || isFak(k)
				visit(k, order)
			case "trd":
				add(k.attr["lng"], bucket, []string{trdText(k)})
			case "trdgrp":
				var ws []string
				for _, t := range k.kids {
					if t.name == "trd" {
						ws = append(ws, trdText(t))
					}
				}
				add(k.attr["lng"], bucket, ws)
			case "ekz", "bld", "rim", "subdrv", "dif", "fnt", "adm", "kap":
				// Translations inside examples translate the example, not the word.
				if k.name == "subdrv" {
					visit(k, bucket)
				}
			}
		}
	}
	visit(dn, 0)
	// Drv-level translations (bucket 0) come first, then the general senses
	// in ReVo's order, then the senses of a special field.
	sort.SliceStable(keys, func(i, j int) bool {
		if fak[keys[i].bucket] != fak[keys[j].bucket] {
			return !fak[keys[i].bucket]
		}
		return keys[i].bucket < keys[j].bucket
	})
	for _, k := range keys {
		var ws []string
		seen := map[string]bool{}
		for _, w := range got[k] {
			if w != "" && !seen[w] {
				seen[w] = true
				ws = append(ws, w)
			}
		}
		if len(ws) > 0 {
			d.trd[k.lng] = append(d.trd[k.lng], ws)
		}
	}
}

// isFak reports whether a sense belongs to a special field (<uzo tip="fak">).
func isFak(snc *node) bool {
	for _, k := range snc.kids {
		if k.name == "uzo" && k.attr["tip"] == "fak" {
			return true
		}
	}
	return false
}

// trdText is a translation's display text: clarifications that ReVo shows
// only in its index (klr tip="ind"), ambiguity notes (tip="amb") and
// pronunciations are dropped.
// entities are the DTD's entities, for leftovers: some articles escape a
// character entity twice ("&amp;t_-a;" in Tibetan translations), so it
// reaches the text as "&t_-a;" and is resolved here.
var (
	entities  map[string]string
	leftEntRe = regexp.MustCompile(`&([A-Za-z0-9_.-]+);`)
)

func resolveLeftovers(s string) string {
	return leftEntRe.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := entities[m[1:len(m)-1]]; ok {
			return v
		}
		return m
	})
}

func trdText(n *node) string {
	var b strings.Builder
	var rec func(n *node)
	rec = func(n *node) {
		for _, k := range n.kids {
			switch k.name {
			case "":
				b.WriteString(k.text)
			case "klr":
				if t := k.attr["tip"]; t != "ind" && t != "amb" {
					rec(k)
				}
			case "pr", "fnt", "ofc", "uzo":
			default:
				rec(k)
			}
		}
	}
	rec(n)
	return squash(resolveLeftovers(b.String()))
}

// heads expands a drv's <kap>: the tilde is the root, variants and
// comma-separated forms each become a headword.
func heads(kap *node, rad string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		for _, p := range strings.Split(b.String(), ",") {
			p = strings.TrimSpace(strings.ReplaceAll(p, "/", ""))
			if p != "" {
				out = append(out, p)
			}
		}
		b.Reset()
	}
	var rec func(n *node)
	rec = func(n *node) {
		for _, k := range n.kids {
			switch k.name {
			case "":
				b.WriteString(k.text)
			case "tld":
				b.WriteString(tilde(rad, k.attr["lit"]))
			case "var":
				flush()
				if vk := child(k, "kap"); vk != nil {
					rec(vk)
				}
				flush()
			case "fnt", "ofc":
			default:
				rec(k)
			}
		}
	}
	rec(kap)
	flush()
	for i, h := range out {
		out[i] = squash(h)
	}
	return out
}

func tilde(rad, lit string) string {
	if lit == "" {
		return rad
	}
	_, n := utf8.DecodeRuneInString(rad)
	return lit + rad[n:]
}

// firstDef is the drv's first <dif> (its own or a sense's), without
// examples, sources and remarks; a general sense wins over one marked
// with a field (uzo tip="fak": the zoological genus Homo before "human").
func firstDef(dn *node, rad string) string {
	var dif, fakDif *node
	var visit func(n *node, fak bool)
	visit = func(n *node, fak bool) {
		for _, k := range n.kids {
			if dif != nil {
				return
			}
			switch k.name {
			case "dif":
				if fak {
					if fakDif == nil {
						fakDif = k
					}
				} else {
					dif = k
				}
			case "snc", "subsnc":
				visit(k, fak || isFak(k))
			}
		}
	}
	visit(dn, false)
	if dif == nil {
		dif = fakDif
	}
	if dif == nil {
		return ""
	}
	var b strings.Builder
	var rec func(n *node)
	rec = func(n *node) {
		for _, k := range n.kids {
			switch k.name {
			case "":
				b.WriteString(k.text)
			case "tld":
				b.WriteString(tilde(rad, k.attr["lit"]))
			case "ekz", "fnt", "rim", "trdgrp", "bld", "adm", "url":
			default:
				rec(k)
			}
		}
	}
	rec(dif)
	s := squash(emptyBrackets.ReplaceAllString(b.String(), ""))
	s = strings.TrimRight(s, ":;, ")
	if s != "" && !strings.HasSuffix(s, ".") {
		s += "."
	}
	if s == "." {
		return ""
	}
	return s
}

var emptyBrackets = regexp.MustCompile(`\s*\(\s*\)|\s*\[\s*\]`)

func plain(n *node) string {
	var b strings.Builder
	walk(n, func(k *node) bool {
		if k.name == "" {
			b.WriteString(k.text)
		}
		return true
	})
	return b.String()
}

func walk(n *node, f func(*node) bool) {
	for _, k := range n.kids {
		if f(k) {
			walk(k, f)
		}
	}
}

func find(n *node, name string) *node {
	var got *node
	walk(n, func(k *node) bool {
		if got != nil {
			return false
		}
		if k.name == name {
			got = k
			return false
		}
		return true
	})
	return got
}

func child(n *node, name string) *node {
	for _, k := range n.kids {
		if k.name == name {
			return k
		}
	}
	return nil
}

func squash(s string) string {
	s = strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
	s = strings.ReplaceAll(s, " ,", ",")
	s = strings.ReplaceAll(s, " .", ".")
	s = strings.ReplaceAll(s, "( ", "(")
	s = strings.ReplaceAll(s, " )", ")")
	return strings.TrimSpace(s)
}
