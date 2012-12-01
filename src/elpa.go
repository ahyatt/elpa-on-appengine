// Copyright 2012 Google Inc. All Rights Reserved.
//
// 	Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fileimport

import (
	"appengine"
	"appengine/blobstore"
	"appengine/datastore"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func init() {
	http.HandleFunc("/upload", upload)
	http.HandleFunc("/md", metadata)
	http.HandleFunc("/mdset", metadataset)
	http.HandleFunc("/packages/archive-contents", archivecontents)
	http.HandleFunc("/packages/", packages)
	http.HandleFunc("/", main)
}

type Package struct {
	Name          string `datastore:name`
	Description   string `datastore:description,noindex`
	Readme        string `datastore:readme,noindex`
	LatestVersion string `datastore:contentid,noindex`
}

type Contents struct {
	BlobKey    appengine.BlobKey `datastore:data`
	Version    string            `datastore:version`
	UploadTime time.Time         `datastore:uploadtime`
}

func upload(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	blobs, values, err := blobstore.ParseUpload(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	file := blobs["file"]
	if len(file) == 0 {
		c.Errorf("No file uploaded")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Redirect(w, r,
		"/md?blobKey="+string(file[0].BlobKey)+
			"&name="+values.Get("name"), http.StatusFound)
}

func packageKey(c appengine.Context, name string) *datastore.Key {
	return datastore.NewKey(c, "Package", name, 0, nil)
}

func versionKey(c appengine.Context, version string, packageKey *datastore.Key) *datastore.Key {
	return datastore.NewKey(c, "Contents", version, 0, packageKey)
}

type PackageAndBlobKey struct {
	Package Package
	BlobKey string
}

func metadata(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	blobkey := r.FormValue("blobKey")
	var p Package
	if len(name) > 0 {
		c := appengine.NewContext(r)
		err := datastore.Get(c, packageKey(c, name), &p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		p = Package{}
	}
	err := templates.ExecuteTemplate(w, "md", PackageAndBlobKey{Package: p, BlobKey: blobkey})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func metadataset(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	name := r.FormValue("name")
	key := packageKey(c, name)
	version := r.FormValue("version")
	var p Package
	err := datastore.Get(c, key, &p)
	if err != nil && err != datastore.ErrNoSuchEntity {
		c.Errorf(fmt.Sprintf("Failed to retrieve package %v", name))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p.Name = name
	p.Description = r.FormValue("description")
	p.Readme = r.FormValue("readme")
	p.LatestVersion = version
	_, err = datastore.Put(c, key, &p)
	if err != nil {
		c.Errorf(fmt.Sprintf("Failed to save package %v", name))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	contents := Contents{
		BlobKey:    appengine.BlobKey(r.FormValue("blobkey")),
		Version:    version,
		UploadTime: time.Now().UTC(),
	}
	_, err = datastore.Put(c, versionKey(c, version, key), &contents)
	if err != nil {
		c.Errorf(
			fmt.Sprintf(
				"Failed to save contents for version %v, package %v",
				version, name))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func main(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	uploadURL, err := blobstore.UploadURL(c, "/upload", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	q := datastore.NewQuery("Package")
	var packages []*Package
	_, err = q.GetAll(c, &packages)
	w.Header().Set("Content-Type", "text/html")
	templateData := struct {
		UploadURL string
		Packages  []*Package
	}{uploadURL.String(), packages}
	err = templates.ExecuteTemplate(w, "main", templateData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func archivecontents(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	q := datastore.NewQuery("Package")
	var packages []*Package
	_, err := q.GetAll(c, &packages)
	w.Header().Set("Content-Type", "text/plain")
	err = archiveContentsTemplate.Execute(w, packages)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var readmeRE = regexp.MustCompile("-readme.txt$")
var nameVersionRE = regexp.MustCompile("([a-z\\-]+)([\\d\\.]+).el")

func packages(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	w.Header().Set("Content-Type", "text/plain")
	file := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	if readmeRE.MatchString(file) {
		name := file[:strings.LastIndex(file, "-")]
		var p Package
		err := datastore.Get(c, packageKey(c, name), &p)
		if err != nil {
			http.Error(w, err.Error(),
				http.StatusInternalServerError)
			return
		}
		if len(p.Readme) == 0 {
			fmt.Fprintf(w, "%v", p.Description)
		} else {
			// These \r's will show up as "^M" in the emacs buffer.
			// We don't want that, although hopefully package.el will
			// eventually fix this.
			fmt.Fprintf(w, "%v", strings.Replace(p.Readme, "\r", "", -1))
		}
	} else {
		parts := nameVersionRE.FindStringSubmatch(file)
		if len(parts) < 3 {
			http.Error(w, "Invalid package name",
				http.StatusInternalServerError)
			return
		}
		name := parts[1][:len(parts[1])-1]
		version := parts[2]
		q := datastore.NewQuery("Contents").Filter("Version=", version).
			Ancestor(packageKey(c, name))
		for cursor := q.Run(c); ; {
			var contents Contents
			_, err := cursor.Next(&contents)
			if err == datastore.Done {
				break
			}
			if err != nil {
				http.Error(w, err.Error(),
					http.StatusInternalServerError)
				return
			}
			blobstore.Send(w, appengine.BlobKey(contents.BlobKey))
		}
	}
}

func versionList(version string) string {
	parts := strings.Split(version, ".")
	return "(" + strings.Join(parts, " ") + ")"
}

var templates = template.Must(template.ParseGlob("templates/*"))
var archiveContentsTemplate = template.Must(template.New("ArchiveContents").
	Funcs(template.FuncMap{"versionList": versionList}).
	Parse(archiveContentsElisp))

var archiveContentsElisp = `(1 {{range .}}
({{.Name}} . [{{versionList .LatestVersion}} nil "{{.Description}}" single]){{end}})
`
