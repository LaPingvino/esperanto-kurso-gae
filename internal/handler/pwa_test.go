package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceWorkerCarriesTheDeploy(t *testing.T) {
	t.Setenv("EK_DEPLOY", "d42")
	rec := httptest.NewRecorder()
	ServiceWorker(rec, httptest.NewRequest("GET", "/sw.js", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `const VERSION = "d42-`+AssetVersion+`"`) {
		t.Errorf("sw.js lacks the deploy's version:\n%.300s", body)
	}
	if strings.Contains(body, "__") {
		t.Errorf("sw.js has an unfilled placeholder")
	}
	if !strings.Contains(body, `/static/eo.js?v=`+AssetVersion) {
		t.Errorf("sw.js does not precache this deploy's eo.js")
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("sw.js Cache-Control %q, want no-cache", cc)
	}
}

func TestManifest(t *testing.T) {
	var m struct {
		Name, ShortName, StartURL, Scope, Display string
		Icons                                     []struct{ Src, Sizes, Purpose string }
	}
	rec := httptest.NewRecorder()
	Manifest(rec, httptest.NewRequest("GET", "/manifest.webmanifest", nil))
	b := strings.NewReplacer(`"short_name"`, `"ShortName"`, `"start_url"`, `"StartURL"`).Replace(rec.Body.String())
	if err := json.Unmarshal([]byte(b), &m); err != nil {
		t.Fatal(err)
	}
	if m.Name == "" || m.ShortName == "" || m.StartURL != "/" || m.Scope != "/" || m.Display != "standalone" {
		t.Errorf("manifest: %+v", m)
	}
	need := map[string]bool{"192x192 any": false, "512x512 any": false, "192x192 maskable": false, "512x512 maskable": false}
	for _, i := range m.Icons {
		need[i.Sizes+" "+i.Purpose] = true
	}
	for k, ok := range need {
		if !ok {
			t.Errorf("manifest has no %s icon", k)
		}
	}
}
