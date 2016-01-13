// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/daddye/vips"
)

var AddrPort = ":8080"
var ImagePaths = []string{"/home/fergus/Projects/Photos/imgs/", "/media/fergus/Yume/Photos & Vids/Japan 2014/"}
var ThumbnailPath = "/home/fergus/Projects/Photos/thumbs/"
var ThumbPrefix = "thumb_"

/* Location of static content (js, css, html) */
var staticDir = "../static/"

var options = vips.Options{
	Height:       500,
	Crop:         false,
	Extend:       vips.EXTEND_WHITE,
	Interpolator: vips.BILINEAR,
	Gravity:      vips.CENTRE,
	Quality:      90,
}

type Gallery struct {
	Name string
	Sets []*MediaSet
}

type MediaSet struct {
	Name       string
	Path       string
	Date       time.Time
	DateString string
	Images     []*Image
}

type Image struct {
	Name  string
	Data  []byte
	Type  string
	Index int
}

type Page struct {
	Title string
	Body  []byte
}

func (p *Page) save() error {
	filename := p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(filename string) (*Page, error) {
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: filename[:3], Body: body}, nil
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

// var templates = template.Must(template.ParseFiles(staticDir+"edit.html",
// 	staticDir+"view.html",
// 	staticDir+"img.html")).Delims("<<", ">>")

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	//err := templates.ExecuteTemplate(w, tmpl+".html", []*Page{p})

	t, _ := template.ParseFiles(tmpl + ".html")
	err := t.Execute(w, []*Page{p})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderImgTemplate(w http.ResponseWriter, tmpl string, gallery *Gallery) {
	//err := templates.ExecuteTemplate(w, tmpl+".html", p)
	t := template.New(tmpl+".html").Delims("{{%", "%}}")
	_, err := t.ParseFiles(staticDir + tmpl + ".html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	err = t.Execute(w, gallery)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var validPath = regexp.MustCompile("^/(edit|save|view|viewImg|img|static|thumb|video)/(.*)$")

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
	gallery := &Gallery{Name: ""}
	index := 0
	for _, p := range ImagePaths {
		files, err := ioutil.ReadDir(p)
		if err != nil {
			http.Error(w, "No images found", http.StatusInternalServerError)
		}
		fmt.Printf("FIles: %s %d\n", p, len(files))
		date := files[0].ModTime().Format("02 Jan '06")
		dir, _ := path.Split(p)
		set := &MediaSet{Name: dir, Path: p, DateString: date}

		var imgs []*Image
		for _, img := range files {
			ext := filepath.Ext(img.Name())
			name := img.Name()[:len(img.Name())-len(ext)]
			if strings.Compare(".jpg", strings.ToLower(ext)) == 0 {
				imgs = append(imgs, &Image{Name: name, Type: ext, Index: index})
				index = index + 1
			} else if strings.Compare(".mp4", strings.ToLower(ext)) == 0 && name[0] != '.' {
				imgs = append(imgs, &Image{Name: name, Type: ext, Index: -1})

			}

		}
		set.Images = imgs
		gallery.Sets = append(gallery.Sets, set)
	}

	renderImgTemplate(w, "img", gallery)

}
func thumbHandler(w http.ResponseWriter, r *http.Request, s string) {
	imgHandler(w, r, path.Join(ThumbnailPath, r.URL.Path[7:]))
}

func fullImgHandler(w http.ResponseWriter, r *http.Request, s string) {
	imgHandler(w, r, path.Join("/", s))
}

func imgHandler(w http.ResponseWriter, r *http.Request, imgPath string) {
	if strings.Compare(".jpg", strings.ToLower(filepath.Ext(imgPath))) != 0 {
		http.Error(w, "Invalide filetype", http.StatusBadRequest)
	}
	inBuf, err := ioutil.ReadFile(imgPath)
	if err != nil {
		fmt.Println(err.Error())
	}
	w.Write(inBuf)
}

func videoHandler(w http.ResponseWriter, r *http.Request, vidPath string) {
	if strings.Compare(".mp4", strings.ToLower(filepath.Ext(vidPath))) != 0 {
		http.Error(w, "Invalide filetype", http.StatusBadRequest)
	}
	vid, err := os.Open(path.Join("/", vidPath))
	if err != nil {
		fmt.Println(err.Error())
	}
	http.ServeContent(w, r, vidPath, time.Time{}, vid)
}

func staticHandler(w http.ResponseWriter, r *http.Request, title string) {
	body, err := ioutil.ReadFile(staticDir + r.URL.Path[7:])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Printf("HTTP(%s): %s %s \n\t%s\n", r.Method, r.URL.Path, "404", ("static" + r.URL.Path[7:]))
	}
	w.Write(body)
}

func thumbGenerator(imgName string, imgPath string, thumbPath string) {
	// fmt.Printf("Hi there, I love %s!\n", strings.ToLower(filepath.Ext(r.URL.Path)))
	thumbName := imgName

	inBuf, err := ioutil.ReadFile(path.Join(imgPath, imgName))
	if err != nil {
		fmt.Println(err.Error())
	}
	buf, err := vips.Resize(inBuf, options)
	if err != nil {
		fmt.Printf("Error creating thumbnail for: %s\n", path.Join(imgPath, imgName))
	}

	err = ioutil.WriteFile(path.Join(thumbPath, thumbName), buf, 0775)
	if err != nil {
		fmt.Printf("Error writing thumbnail to disk: %s - %s", thumbPath,
			err.Error())
	}
}

func videoThumbGenerator(vidName string, vidPath string, thumbPath string) {
	thumbName := vidName[:len(vidName)-len(filepath.Ext(vidName))] + ".jpg"
	cmd := exec.Command("bash", "-c", "ffmpeg -ss 00:00:01 -i '"+path.Join(vidPath, vidName)+"' -frames:v 1 '"+path.Join(thumbPath, thumbName)+"'")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error creating videoThumb (%s) %s\n", vidName, err.Error())
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		fmt.Println("ffmpeg -ss 00:00:01 -i " + path.Join(vidPath, vidName) + " -frames:v 1 '" + path.Join(thumbPath, thumbName) + "'")
	}
}

func folderProcessor(folderPath string, thumbPath string) {
	/* Get images to process */
	imgs, err := ioutil.ReadDir(folderPath)
	if err != nil {
		fmt.Printf("Folder Processor Error: %s", err.Error())
	}
	fmt.Printf("Found %d files to process\n", len(imgs))
	/* Check destination dir exists else create it */
	dst := filepath.Join(thumbPath, folderPath)
	fmt.Printf("DST: %s\n", dst)
	_, err = os.Stat(dst)
	if os.IsNotExist(err) {
		fmt.Printf("Thumb directory created: %s\n", dst)
		err = os.MkdirAll(dst, 0755)
		if err != nil {
			fmt.Printf("E:%s", err.Error())
		}
	}
	for index, mediaPath := range imgs {
		fmt.Printf("\r %d/%d - %.2f", index+1, len(imgs), float32(index+1)/float32(len(imgs))*100)
		ext := strings.ToLower(filepath.Ext(mediaPath.Name()))
		if strings.Compare(".jpg", ext) == 0 {
			thumbGenerator(mediaPath.Name(), folderPath, dst)
		} else if strings.Compare(".mp4", ext) == 0 && mediaPath.Name()[0] != '.' {
			videoThumbGenerator(mediaPath.Name(), folderPath, dst)
		}
	}
	fmt.Printf("Processing Complete\n")
}

func main() {
	fmt.Print("Configuration:\n")
	fmt.Printf(">> Runtime: %s - %s - %s\n", runtime.GOOS, runtime.Compiler, runtime.Version())
	fmt.Printf(">> CPU(%s): %d cores\n", runtime.GOARCH, runtime.NumCPU())
	fmt.Printf(">> Static Dir: %s\n", staticDir)
	runtime.GOMAXPROCS(runtime.NumCPU())
	//folderProcessor(ImagePaths[1], ThumbnailPath)
	fmt.Printf("Starting Web Server on %s\n", AddrPort)
	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	http.HandleFunc("/img/", makeHandler(fullImgHandler))
	http.HandleFunc("/thumb/", makeHandler(thumbHandler))
	http.HandleFunc("/viewImg/", makeHandler(viewImgHandler))
	// http.HandleFunc("/static/", makeHandler(staticHandler))
	http.HandleFunc("/video/", makeHandler(videoHandler))
	http.ListenAndServe(":8080", nil)
}
