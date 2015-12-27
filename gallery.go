// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"regexp"
)

var path = "imgs"

type Page struct {
	Title string
	Body  []byte
}

func (p *Page) save() error {
	filename := p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

var templates = template.Must(template.ParseFiles("edit.html", "view.html", "img.html"))

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", []*Page{p})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderImgTemplate(w http.ResponseWriter, tmpl string, p []*Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var validPath = regexp.MustCompile("^/(edit|save|view|viewImg|img|static)/(([a-zA-Z0-9]|\\.|[[:space:]]|.)*)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		fmt.Printf("HTTP(%s): %s\n", r.Method, r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func viewImgHandler(w http.ResponseWriter, r *http.Request, s string) {
	//fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[5:])
	//body, err := ioutil.ReadFile(r.URL.Path[5:])
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// }
	//w.Write(body)
	var pages []*Page
	imgs, err := ioutil.ReadDir(path)
	if err != nil {
		http.Error(w, "No images found", http.StatusInternalServerError)
	}
	for _, img := range imgs {
		pages = append(pages, &Page{Title: img.Name()})
	}
	renderImgTemplate(w, "img", pages)

}

func imgHandler(w http.ResponseWriter, r *http.Request, s string) {
	//fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[5:])
	body, err := ioutil.ReadFile(path + "/" + r.URL.Path[5:])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("HTTP(%s): %s %s\n", r.Method, r.URL.Path, "404")
	}
	w.Write(body)
}

func staticHandler(w http.ResponseWriter, r *http.Request, title string) {
	body, err := ioutil.ReadFile("js" + "/" + r.URL.Path[7:])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("HTTP(%s): %s %s\n", r.Method, r.URL.Path, "404")
	}
	w.Write(body)
}

func main() {
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	http.HandleFunc("/img/", makeHandler(imgHandler))
	http.HandleFunc("/viewImg/", makeHandler(viewImgHandler))
	http.HandleFunc("/static/", makeHandler(staticHandler))
	http.ListenAndServe(":8080", nil)
}
