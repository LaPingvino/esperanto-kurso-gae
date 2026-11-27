package locale

import (
	"strings"
	"testing"
)

// Every locale uses only keys the Esperanto file has (a key elsewhere is a
// typo or a leftover), apart from the draft marker, which is per locale.
func TestLocaleKeysKnown(t *testing.T) {
	eo := translations["eo"]
	for code, m := range translations {
		for k := range m {
			if strings.HasPrefix(k, "locale.") {
				continue
			}
			if _, ok := eo[k]; !ok {
				t.Errorf("%s.json has key %q, which eo.json lacks", code, k)
			}
		}
	}
}

// A draft locale carries both its line and the line's link text.
func TestDraftLocales(t *testing.T) {
	if !IsDraft("tet") {
		t.Error("tet should mark itself a draft")
	}
	if IsDraft("eo") || IsDraft("en") {
		t.Error("eo and en are not drafts")
	}
	for code, m := range translations {
		if IsDraft(code) && m["locale.draft_link"] == "" {
			t.Errorf("%s is a draft but has no locale.draft_link", code)
		}
	}
	if DetectLang("tet-TL,pt;q=0.8") != "tet" {
		t.Error("Accept-Language tet-TL should pick tet")
	}
}
