package handler

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/legvortoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

func lvItems() map[string]*model.ContentItem {
	v := func(slug, word, author string, rating float64, tags ...string) *model.ContentItem {
		return &model.ContentItem{Slug: slug, Type: "vocab", Status: "approved", AuthorID: author, Rating: rating, RD: 150, Volatility: 0.06,
			Content: map[string]any{"word": word}, Tags: tags}
	}
	return map[string]*model.ContentItem{
		"lit-la-eta-princo-komenco": {Slug: "lit-la-eta-princo-komenco", Type: "reading", Status: "approved", AuthorID: "seed",
			Content: map[string]any{"title": "La eta princo", "text": "Tiu bildo prezentis boaon. La boaj glutas sian rabaĵon. Mi desegnis boaojn kaj librojn."}},
		"bildo-libro":       v("bildo-libro", "libro", "seed", 900, "bildvortaro"),
		"voc-auto-libro":    v("voc-auto-libro", "libro", "", 1600, "vortaro", "lit-la-eta-princo-komenco"),
		"voc-auto-boa":      v("voc-auto-boa", "boa", "", 2100, "vortaro", "lit-la-eta-princo-komenco"),
		"voc-auto-boao":     v("voc-auto-boao", "boao", "", 1700, "vortaro", "lit-la-eta-princo-komenco"),
		"voc-auto-desegnis": v("voc-auto-desegnis", "desegnis", "", 1600, "vortaro", "lit-la-eta-princo-komenco"),
	}
}

func snapshotStore(m map[string]*model.ContentItem) map[string]string {
	out := map[string]string{}
	for k, it := range m {
		b, _ := json.Marshal(it)
		out[k] = string(b)
	}
	return out
}

func TestLegvortojPlanApplyUndo(t *testing.T) {
	_, fc, mux := newTestBuilder()
	fc.m = lvItems()
	before := snapshotStore(fc.m)

	if w := call(mux, "GET", "/builder-api/legvortoj", "", ""); w.Code != 401 {
		t.Fatalf("no key: %d", w.Code)
	}
	w := call(mux, "GET", "/builder-api/legvortoj", testKey, "")
	if w.Code != 200 {
		t.Fatalf("plan: %d %s", w.Code, w.Body)
	}
	var plan struct {
		Fingerprint string
		Counts      map[string]int
		Actions     []legvortoj.Action
	}
	json.Unmarshal(w.Body.Bytes(), &plan)
	if !reflect.DeepEqual(snapshotStore(fc.m), before) {
		t.Fatal("the plan changed the store")
	}
	if plan.Counts["core"] != 1 || plan.Counts["lemma"] != 2 || plan.Counts["keep"] != 1 || plan.Counts["create"] != 1 {
		t.Fatalf("counts %v: %+v", plan.Counts, plan.Actions)
	}

	if w := call(mux, "POST", "/builder-api/legvortoj/apply", testKey, `{"fingerprint":"nope"}`); w.Code != 409 {
		t.Fatalf("a stale fingerprint: %d", w.Code)
	}
	w = call(mux, "POST", "/builder-api/legvortoj/apply", testKey, `{"fingerprint":"`+plan.Fingerprint+`"}`)
	if w.Code != 200 {
		t.Fatalf("apply: %d %s", w.Code, w.Body)
	}
	var res struct {
		Run     string
		Changed int
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	if res.Changed != 5 || !strings.HasPrefix(res.Run, "lv-") {
		t.Fatalf("apply: %s", w.Body)
	}
	// only voc-auto cards changed; the reading and the core card did not
	for _, s := range []string{"lit-la-eta-princo-komenco", "bildo-libro"} {
		b, _ := json.Marshal(fc.m[s])
		if string(b) != before[s] {
			t.Errorf("%s changed", s)
		}
	}
	if it := fc.m["voc-auto-libro"]; it.Status != legvortoj.MergedStatus || it.Content[legvortoj.MergedIntoKey] != "bildo-libro" {
		t.Errorf("libro: %s %v", it.Status, it.Content)
	}
	if it := fc.m["voc-auto-boa"]; it.Status != legvortoj.MergedStatus || it.Content[legvortoj.MergedIntoKey] != "voc-auto-boao" {
		t.Errorf("boa: %s %v", it.Status, it.Content)
	}
	if it := fc.m["voc-auto-boao"]; it.Status != "approved" || it.Rating != 1500 || it.RD != 200 || !reflect.DeepEqual(it.Tags, []string{"vortaro", "legvorto", "lit-la-eta-princo-komenco"}) {
		t.Errorf("boao kept: %+v", it)
	}
	if it := fc.m["voc-auto-desegni"]; it == nil || it.Status != "approved" || it.AuthorID != BuilderOrigin {
		t.Errorf("desegni not made: %+v", it)
	}
	// a second plan has nothing left
	w = call(mux, "GET", "/builder-api/legvortoj", testKey, "")
	json.Unmarshal(w.Body.Bytes(), &plan)
	if len(plan.Actions) != 0 {
		t.Errorf("planned again: %+v", plan.Actions)
	}

	w = call(mux, "POST", "/builder-api/legvortoj/undo", testKey, `{"run":"`+res.Run+`"}`)
	if w.Code != 200 {
		t.Fatalf("undo: %d %s", w.Code, w.Body)
	}
	after := snapshotStore(fc.m)
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s not put back:\n was %s\n now %s", k, v, after[k])
		}
	}
	if it := fc.m["voc-auto-desegni"]; it.Status != legvortoj.MergedStatus {
		t.Errorf("the made card not hidden again: %s", it.Status)
	}
	// and it can be done again after the undo
	w = call(mux, "GET", "/builder-api/legvortoj", testKey, "")
	json.Unmarshal(w.Body.Bytes(), &plan)
	if w := call(mux, "POST", "/builder-api/legvortoj/apply", testKey, `{"fingerprint":"`+plan.Fingerprint+`"}`); w.Code != 200 {
		t.Fatalf("apply again: %d %s", w.Code, w.Body)
	}
	if it := fc.m["voc-auto-desegni"]; it.Status != "approved" {
		t.Errorf("the made card not shown again: %s", it.Status)
	}
}

func TestStandalone(t *testing.T) {
	shared := &model.ContentItem{Slug: "voc-auto-boao", Type: "vocab", Tags: []string{"vortaro", "legvorto", "lit-la-eta-princo-komenco"}}
	core := &model.ContentItem{Slug: "bildo-libro", Type: "vocab"}
	if standalone(shared, nil) || !standalone(core, nil) {
		t.Error("signed out: only core cards")
	}
	if !standalone(shared, map[string]bool{"lit-la-eta-princo-komenco": true}) {
		t.Error("read the text: its words too")
	}
	if got := keepStandalone([]*model.ContentItem{shared, core}, map[string]bool{"other": true}); len(got) != 1 || got[0] != core {
		t.Errorf("%v", got)
	}
}

func TestGlossaryAndPractice(t *testing.T) {
	reading := &model.ContentItem{Slug: "lit-la-eta-princo-komenco", Type: "reading",
		Content: map[string]any{"text": "Tiu bildo prezentis boaon, kiu glutas rabobeston. La boaj glutas sian rabaĵon unuglute. Mi pripensis la aventurojn de la ĝangalo."}}
	x := legvortoj.NewIndex([]*model.ContentItem{{Slug: "bildo-bildo", Type: "vocab", Status: "approved", Content: map[string]any{"word": "bildo"}}})
	words := Glossary(reading, x, "de")
	var boao *GlossWord
	for _, g := range words {
		if g.Lemma == "boao" {
			boao = g
		}
		if g.Lemma == "bildo" && g.New {
			t.Error("bildo is a core word")
		}
	}
	if boao == nil || !boao.New || strings.Contains(boao.M.Text, "Schal") || !strings.Contains(boao.M.EoDef, "serpent") {
		t.Fatalf("boao: %+v", boao)
	}
	if !strings.Contains(boao.HintDef(), "~edoj") {
		t.Errorf("hint gives the word away: %q", boao.HintDef())
	}
	html := string(GlossText(reading.Text(), words))
	if !strings.Contains(html, `<a class="eo-word" href="#v-boao" data-v="v-boao">boaj</a>`) {
		t.Errorf("boaj not linked to boao: %s", html)
	}
	qs := Practice(reading.Slug, words)
	if len(qs) == 0 {
		t.Fatal("no practice")
	}
	again := Practice(reading.Slug, words)
	for i, q := range qs {
		if !containsStr(q.Options, q.Form) || len(q.Options) < 2 || q.Before+q.Form+q.After != q.Word.Sentence && !strings.EqualFold(q.Before+q.Form+q.After, q.Word.Sentence) {
			t.Errorf("question %d: %+v", i, q)
		}
		if strings.Join(q.Options, ",") != strings.Join(again[i].Options, ",") {
			t.Errorf("question %d not the same twice", i)
		}
	}
}

// TestLegvortojBatches: a plan larger than a batch goes in several calls of
// one run, and one undo puts all of it back.
func TestLegvortojBatches(t *testing.T) {
	old := lvBatch
	lvBatch = 2
	defer func() { lvBatch = old }()
	_, fc, mux := newTestBuilder()
	fc.m = lvItems()
	before := snapshotStore(fc.m)
	var res struct {
		Fingerprint string
		Run         string
		Remaining   int
	}
	w := call(mux, "GET", "/builder-api/legvortoj", testKey, "")
	json.Unmarshal(w.Body.Bytes(), &res)
	body := `{"fingerprint":"` + res.Fingerprint + `"}`
	calls := 0
	for {
		calls++
		w = call(mux, "POST", "/builder-api/legvortoj/apply", testKey, body)
		if w.Code != 200 {
			t.Fatalf("apply %d: %d %s", calls, w.Code, w.Body)
		}
		res.Remaining, res.Fingerprint = 0, ""
		json.Unmarshal(w.Body.Bytes(), &res)
		if res.Remaining == 0 {
			break
		}
		body = `{"fingerprint":"` + res.Fingerprint + `","run":"` + res.Run + `"}`
		if calls > 5 {
			t.Fatal("no end")
		}
	}
	if calls != 3 {
		t.Errorf("%d calls for 5 actions in batches of 2", calls)
	}
	w = call(mux, "GET", "/builder-api/legvortoj", testKey, "")
	var plan struct{ Actions []legvortoj.Action }
	json.Unmarshal(w.Body.Bytes(), &plan)
	if len(plan.Actions) != 0 {
		t.Errorf("left: %+v", plan.Actions)
	}
	call(mux, "POST", "/builder-api/legvortoj/undo", testKey, `{"run":"`+res.Run+`"}`)
	after := snapshotStore(fc.m)
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s not put back", k)
		}
	}
	if w := call(mux, "POST", "/builder-api/legvortoj/apply", testKey, `{"fingerprint":"x","run":"../etc"}`); w.Code != 409 && w.Code != 400 {
		t.Errorf("a bad run id: %d", w.Code)
	}
}

// TestNoNearWordMeaning: a word ReVo lacks gets no meaning of a word that
// only looks near (akrega is akr|eg|a, not akreo, an acre; ele is el|e,
// not elo, the letter L); a form of a word gets that word's.
func TestNoNearWordMeaning(t *testing.T) {
	for _, w := range []string{"akrega", "ele"} {
		m := MeaningFor(&model.ContentItem{Type: "vocab", Content: map[string]any{"word": w}}, "en", "")
		if m.Text != "" || m.ReVoFound {
			t.Errorf("%s: %q (via %q)", w, m.Text, m.Via)
		}
	}
	if m := MeaningFor(&model.ContentItem{Type: "vocab", Content: map[string]any{"word": "aĉetita"}}, "en", ""); !strings.Contains(m.Text, "buy") || m.Via != "aĉeti" {
		t.Errorf("aĉetita: %q via %q", m.Text, m.Via)
	}
}
