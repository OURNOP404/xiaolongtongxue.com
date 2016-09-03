package controller

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/longkai/xiaolongtongxue.com/config"
	"github.com/longkai/xiaolongtongxue.com/github"
	"github.com/longkai/xiaolongtongxue.com/render"
)

const (
	pageSize = 7
)

var (
	templs = template.Must(template.New(`sakura`).Funcs(template.FuncMap{
		`tags`:     render.Tags,
		`format`:   render.Format,
		`daysAgo`:  render.DaysAgo,
		`hasColor`: render.HasColor,
		`hasImage`: render.HasImage,
		`relImage`: render.IsRelImage,
	}).ParseGlob(`templ/*`))

	sakura   render.Engine
	staticFs http.Handler
)

// Ctrl main controller.
func Ctrl() {
	sakura = render.NewSakura()
	staticFs = http.FileServer(http.Dir(config.Env.Repo))
	sakura.Post(config.Env.Repo)

	github.Init(`/api/github/hook`, config.Env.Repo, config.Env.HookSecret, config.Env.AccessToken, revalidate)
	for _, v := range config.Roots() {
		installHanlder(v)
	}

	http.HandleFunc("/", home)
	http.HandleFunc("/ls/", ls)
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
}

var installHanlder = func(p string) {
	p = fmt.Sprintf("/%s/", p)
	log.Printf("mapping url %s*", p)
	http.HandleFunc(p, entry)
}

var revalidate = func(a, m, d []string) {
	for i := range a {
		if v := config.Root(filepath.Join(config.Env.Repo, a[i])); v != "" {
			installHanlder(v)
		}
	}
	if err := sakura.Revalidate(a, m, d); err != nil {
		log.Printf("revalidate fail: %v", err)
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI != "/" {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}

	v, err := sakura.Ls("", pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := &struct {
		List interface{}
		Meta interface{}
	}{v, config.Env.Meta}
	if err := templs.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func entry(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.RequestURI, "/") {
		staticFs.ServeHTTP(w, r)
		return
	}

	v, err := sakura.Get(r.RequestURI)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data := &struct {
		A    interface{}
		Meta interface{}
	}{v, config.Env.Meta}
	if err = templs.ExecuteTemplate(w, "entry.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func ls(w http.ResponseWriter, r *http.Request) {
	key := r.RequestURI[len("/ls"):]
	if len(key) <= 1 { // `/` is not allowed
		http.Error(w, fmt.Sprintf("RequestURI %q, last segment not found", r.RequestURI), http.StatusBadRequest)
		return
	}
	v, err := sakura.Ls(key, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
}
