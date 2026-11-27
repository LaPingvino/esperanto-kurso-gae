package handler

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

// The site as an installable app: the web manifest and the service worker
// (sw.js; why it never serves old code after a deploy is said there).

//go:embed sw.js
var swSource string

//go:embed manifest.webmanifest
var manifest []byte

// AssetFiles are the /static/ files every page loads with ?v=AssetVersion
// (their hash): the service worker keeps them ahead.
var AssetFiles = []string{"tokens.css", "stilo.css", "eo.js", "fsrs.js", "ripetoj.js", "eo-ripetoj.js"}

// DeployVersion names this deploy: the service worker's caches carry it, so
// each deploy installs a new worker that drops the old caches. App Engine
// gives each deploy its own GAE_VERSION; EK_DEPLOY overrides it (a test can
// fake a deploy); the static files' hash keeps a local run honest.
func DeployVersion() string {
	v := os.Getenv("EK_DEPLOY")
	if v == "" {
		v = os.Getenv("GAE_VERSION")
	}
	if v == "" {
		v = "loka"
	}
	return v + "-" + AssetVersion
}

// ServiceWorker serves /sw.js: at the root, so its scope is the whole site.
func ServiceWorker(w http.ResponseWriter, r *http.Request) {
	pre := []string{}
	for _, f := range AssetFiles {
		pre = append(pre, "/static/"+f+"?v="+AssetVersion)
	}
	pj, _ := json.Marshal(pre)
	vj, _ := json.Marshal(DeployVersion())
	src := strings.Replace(swSource, `"__VERSION__"`, string(vj), 1)
	src = strings.Replace(src, "__PRECACHE__", string(pj), 1)
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(src))
}

// Manifest serves /manifest.webmanifest.
func Manifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(manifest)
}
