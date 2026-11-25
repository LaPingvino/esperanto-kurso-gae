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
		Source:  "Reta Vortaro (reta-vortaro.de), github.com/revuloj/revo-fonto, GPL-2.0",
		Roots:   map[string]int{},
		RootOfc: map[string]string{},
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
			e := revo.Entry{Mrk: dv.mrk, Heads: dv.heads, Def: dv.def, Trd: revo.EncodeTrd(trd), Ofc: dv.ofc, Senses: dv.senses}
			if len(e.Heads) == 0 {
				continue
			}
			d.Drv = append(d.Drv, e)
		}
		if len(d.Drv) > first {
			for _, r := range art.roots {
				if _, ok := d.Roots[r]; !ok {
					d.Roots[r] = first
					if art.ofc != "" {
						d.RootOfc[r] = art.ofc
					}
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
	ofc   string // <ofc> in the drv's <kap>: "*" Fundamento, "1"… an Official Addition
	mrk   string
	heads []string
	def   string
	trd   map[string][][]string // lng → senses → words
	// senses: the drv's senses, when it has more than one, each with its
	// own definition and translations, so a text can name the one it means.
	senses []revo.Sense
}

type article struct {
	ofc   string // the root's <ofc>, in the article's <kap>
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
	if o := child(kap, "ofc"); o != nil {
		a.ofc = strings.TrimSpace(plain(o))
	}
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
					if o := find(h, "ofc"); o != nil {
						d.ofc = strings.TrimSpace(plain(o))
					}
				}
				d.def = firstDef(k, radText)
				addSenses(k, d, radText)
				d.senses = senseList(k, radText)
				a.drvs = append(a.drvs, d)
			case "subart":
				collect(k)
			}
		}
	}
	collect(art)
	return a, nil
}

// addSenses fills d.trd: per language, the translations of the word's
// main sense. ReVo puts translations directly under the drv (they name
// the word as a whole) and under each sense (snc/subsnc); a word's senses
// can be far apart (boao: the snake, and a fur scarf, TEKS). The main
// sense is ReVo's first sense that is not a figurative or other styled
// use (FIG, ARK, POE: <uzo tip="stl">). A language gets its drv-level
// translations and the main sense's; without drv-level ones, the first
// sense (main first, styled ones last) that has one in it. Later senses are
// left out — they are other meanings — and so is a drv-level translation
// narrowed by a clarification ("boa (kožušina)") in a word of several
// senses, since it names one sense, not the word.
func addSenses(dn *node, d *drv, rad string) {
	var visit func(n *node, bucket int)
	order := 0
	type key struct {
		lng    string
		bucket int
	}
	styled := map[int]bool{}
	taxon := map[int]bool{}
	fakTop := map[int]bool{}
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
	senses := 0
	top := map[int]int{}          // bucket → its top-level sense's bucket
	narrowed := map[string]bool{} // drv-level translations with a clarification
	visit = func(n *node, bucket int) {
		for _, k := range n.kids {
			switch k.name {
			case "snc", "subsnc":
				order++
				if bucket == 0 {
					senses++
					top[order] = order
				} else {
					top[order] = top[bucket]
				}
				styled[order] = styled[bucket] || isStyled(k)
				taxon[order] = isTaxon(k, rad)
				fakTop[order] = isFak(k)
				visit(k, order)
			case "trd":
				t := trdText(k)
				if bucket == 0 && clarified(k) {
					narrowed[t] = true
				}
				add(k.attr["lng"], bucket, []string{t})
			case "trdgrp":
				var ws []string
				for _, t := range k.kids {
					if t.name == "trd" {
						w := trdText(t)
						if bucket == 0 && clarified(t) {
							narrowed[w] = true
						}
						ws = append(ws, w)
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
	// the main sense: ReVo's first not styled; when that is the taxon, a
	// later plain sense (homo: the genus Homo, then the human being), if any
	main := 0
	for b := 1; b <= order; b++ {
		if top[b] == b && !styled[b] {
			main = b
			break
		}
	}
	if main != 0 && taxon[main] {
		for b := main + 1; b <= order; b++ {
			if top[b] == b && !styled[b] && !taxon[b] && !fakTop[b] {
				main = b
				break
			}
		}
	}
	// drv-level first, then the main sense, then the others in ReVo's
	// order, styled ones last
	rank := func(b int) int {
		switch {
		case b == 0:
			return 0
		case main != 0 && top[b] == main:
			return 1
		case styled[b]:
			return 3
		}
		return 2
	}
	sort.SliceStable(keys, func(i, j int) bool {
		bi, bj := keys[i].bucket, keys[j].bucket
		if rank(bi) != rank(bj) {
			return rank(bi) < rank(bj)
		}
		return bi < bj
	})
	drvLevel := map[string]bool{}
	for _, k := range keys {
		if k.bucket == 0 {
			drvLevel[k.lng] = true
		}
	}
	chosen := map[string]int{} // lang → the top-level sense its translation comes from
	for _, k := range keys {
		if k.bucket != 0 {
			t := top[k.bucket]
			if c, ok := chosen[k.lng]; ok && c != t {
				continue // another sense: another meaning
			}
			if drvLevel[k.lng] && main != 0 && t != main {
				continue // the drv-level translation names the word; only the main sense adds to it
			}
			chosen[k.lng] = t
		}
		var ws []string
		seen := map[string]bool{}
		for _, w := range got[k] {
			if k.bucket == 0 && senses > 1 && narrowed[w] {
				continue
			}
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

// senseList lists a drv's senses (its snc, also those inside a subdrv),
// each with its definition and its translations (of the sense and its
// subsenses, not of examples); nil for a word of one sense.
func senseList(dn *node, rad string) []revo.Sense {
	var out []revo.Sense
	var sncs []*node
	walk(dn, func(n *node) bool {
		switch n.name {
		case "snc":
			sncs = append(sncs, n)
			return false
		case "subdrv":
			return true
		}
		return false
	})
	if len(sncs) < 2 {
		return nil
	}
	for i, sn := range sncs {
		mrk := sn.attr["mrk"]
		if mrk == "" {
			mrk = fmt.Sprintf("%s.%d", dn.attr["mrk"], i+1)
		}
		trd := map[string][]string{}
		var visit func(n *node)
		visit = func(n *node) {
			for _, k := range n.kids {
				switch k.name {
				case "trd":
					if t := trdText(k); t != "" {
						trd[k.attr["lng"]] = append(trd[k.attr["lng"]], t)
					}
				case "trdgrp":
					for _, t := range k.kids {
						if t.name == "trd" {
							if w := trdText(t); w != "" {
								trd[k.attr["lng"]] = append(trd[k.attr["lng"]], w)
							}
						}
					}
				case "subsnc", "dif":
					visit(k) // a translation may sit in the definition (boao: "Boa", la)
				}
			}
		}
		visit(sn)
		m := map[string]string{}
		for l, ws := range trd {
			if l != "" {
				m[l] = revo.JoinSenses([][]string{ws})
			}
		}
		out = append(out, revo.Sense{Mrk: mrk, Def: firstDef(sn, rad), Fak: isFak(sn), Stl: isStyled(sn), Tax: isTaxon(sn, rad), Trd: revo.EncodeTrd(m)})
	}
	return out
}

// isTaxon reports whether a sense is the biological taxon (homo: "Genro
// el…", the genus Homo) rather than the everyday word, which ReVo often
// lists first.
func isTaxon(snc *node, rad string) bool {
	def := firstDef(snc, rad)
	for _, p := range []string{"Genro", "Familio", "Ordo", "Subordo", "Klaso", "Specio", "Subspecio", "Tribo", "Subfamilio"} {
		if strings.HasPrefix(def, p+" ") || strings.HasPrefix(def, p+",") {
			return true
		}
	}
	return false
}

// isStyled reports whether a sense carries a style mark (<uzo tip="stl">:
// FIG, ARK, POE, EVI…): not the plain meaning.
func isStyled(snc *node) bool {
	for _, k := range snc.kids {
		if k.name == "uzo" && k.attr["tip"] == "stl" {
			return true
		}
	}
	return false
}

// clarified reports whether a translation carries a shown clarification
// (<klr> other than the index-only and ambiguity notes).
func clarified(trd *node) bool {
	found := false
	walk(trd, func(n *node) bool {
		if n.name == "klr" && n.attr["tip"] != "ind" && n.attr["tip"] != "amb" {
			found = true
		}
		return !found
	})
	return found
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
