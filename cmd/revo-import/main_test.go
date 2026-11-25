package main

import (
	"strings"
	"testing"
)

// boao (ReVo boa.xml, shortened): the snake (a general sense and a ZOO
// one) and the fur scarf (TEKS), with drv-level translations, one of them
// narrowed to the scarf. La eta princo's boao is the snake.
const boaXML = `<?xml version="1.0"?>
<vortaro><art mrk="boa">
<kap><ofc>*</ofc><rad>boa</rad>/o</kap>
<drv mrk="boa.0o">
  <kap><ofc>*</ofc><tld/>o</kap>
  <snc mrk="boa.0o.KOMUNE"><dif>Iu el familio de grandaj sufokaj serpentoj.</dif>
    <trdgrp lng="be"><trd>удаў</trd>, <trd>боа</trd></trdgrp>
  </snc>
  <snc mrk="boa.0o.ZOO"><uzo tip="fak">ZOO</uzo><dif>Genro.</dif>
    <trd lng="de">Boa (Schlange)</trd>
    <trd lng="fr">boa <klr tip="ind">(serpent)</klr></trd>
    <trdgrp lng="ru"><trd>боа</trd>, <trd>удав</trd></trdgrp>
  </snc>
  <snc mrk="boa.0o.TEKS"><uzo tip="fak">TEKS</uzo><dif>Longa virina peltaĵo.</dif>
    <trd lng="de">Boa (Schal)</trd>
    <trd lng="fr">boa <klr tip="ind">(écharpe)</klr></trd>
    <trd lng="ru">боа <klr>(шарф)</klr></trd>
  </snc>
  <trd lng="de">Boa</trd>
  <trd lng="en">boa</trd>
  <trd lng="sk">boa <klr>(kožušina)</klr></trd>
</drv>
</art></vortaro>`

func TestMainSenseOnly(t *testing.T) {
	a, err := parseArticle([]byte(boaXML), map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.drvs) != 1 {
		t.Fatalf("%d drvs", len(a.drvs))
	}
	d := a.drvs[0]
	if d.ofc != "*" || a.ofc != "*" {
		t.Errorf("ofc %q, root ofc %q; want *", d.ofc, a.ofc)
	}
	flat := func(l string) string {
		var parts []string
		for _, s := range d.trd[l] {
			parts = append(parts, strings.Join(s, ", "))
		}
		return strings.Join(parts, "; ")
	}
	for l, want := range map[string]string{
		"de": "Boa", // the word; the main sense (the snakes) adds none, the scarf is left out
		"ru": "боа, удав",
		"fr": "boa",
		"en": "boa",
		"be": "удаў, боа",
		"sk": "", // only a drv-level translation narrowed to the scarf
	} {
		if got := flat(l); got != want {
			t.Errorf("%s: %q, want %q", l, got, want)
		}
	}
}

func TestSenseList(t *testing.T) {
	a, err := parseArticle([]byte(boaXML), map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	ss := a.drvs[0].senses
	if len(ss) != 3 {
		t.Fatalf("%d senses, want 3", len(ss))
	}
	if ss[0].Mrk != "boa.0o.KOMUNE" || ss[0].Fak || !strings.Contains(ss[0].Def, "serpentoj") {
		t.Errorf("sense 1: %+v", ss[0])
	}
	if !ss[2].Fak || ss[2].Translation("de") != "Boa (Schal)" || ss[1].Translation("de") != "Boa (Schlange)" {
		t.Errorf("senses 2, 3: %+v %+v", ss[1], ss[2])
	}
}
