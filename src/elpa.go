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

package elpa

import (
	"appengine"
	"appengine/blobstore"
	"appengine/datastore"
	"bufio"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Contents struct {
	BlobKey    appengine.BlobKey `datastore:data`
	Version    string            `datastore:version`
	UploadTime time.Time         `datastore:uploadtime`
}

func init() {
	http.HandleFunc("/upload", upload)
	http.HandleFunc("/packages/archive-contents", archivecontents)
	http.HandleFunc("/packages/", packages)
	http.HandleFunc("/upload.html", uploadInstructions)
	http.HandleFunc("/upload_complete.html", uploadComplete)
	http.HandleFunc("/", main)
}

func uploadInstructions(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	uploadURL, err := blobstore.UploadURL(c, "/upload", nil)
	w.Header().Set("Content-Type", "text/html")
	err = templates.ExecuteTemplate(w, "upload", uploadURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func uploadComplete(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	name := r.FormValue("package")
	var p Package
	err := datastore.Get(c, packageKey(c, name), &p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	details, err := decodeDetails(&p.Details)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if details.Required == nil {
		details.Required = make([]PackageRef, 0)
	}
	templateData := struct {
		Pkg  *Package
		Details  *Details
	}{&p, details}

	err = templates.ExecuteTemplate(w, "upload_complete", templateData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func upload(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	blobs, _, err := blobstore.ParseUpload(r)
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
	reader := blobstore.NewReader(c, file[0].BlobKey)
	pkg, err := parsePackageVarsFromFile(bufio.NewReader(reader))
	if err != nil {
		c.Errorf(fmt.Sprintf("Error reading from upload: %v", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	key := packageKey(c, pkg.Name)
	_, err = datastore.Put(c, key, pkg)
	if err != nil {
		c.Errorf(fmt.Sprintf("Failed to save package %v", pkg.Name))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	contents := Contents{
		BlobKey:    file[0].BlobKey,
		Version:    pkg.LatestVersion,
		UploadTime: time.Now().UTC(),
	}
	_, err = datastore.Put(c, versionKey(c, pkg.LatestVersion, key), &contents)
	if err != nil {
		c.Errorf(
			fmt.Sprintf(
				"Failed to save contents for version %v, package %v",
				pkg.LatestVersion, pkg.Name))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/upload_complete.html?package=" +
		url.QueryEscape(pkg.Name), http.StatusFound)
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

func main(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	q := datastore.NewQuery("Package")
	var packages []*Package
	_, err := q.GetAll(c, &packages)
	w.Header().Set("Content-Type", "text/html")
	err = templates.ExecuteTemplate(w, "main", packages)
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

// Serves several package related urls that package.el expects.
//
// First are readmes, which are served from
// /packages/<package-name>-readme.txt.
//
// Second are package contents, which exist for all uploaded versions
// of a packages. They are servered from
// /packages/<package-name>-<package-version>.el
func packages(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	w.Header().Set("Content-Type", "text/plain")
	file := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	if readmeRE.MatchString(file) {
		name := file[:strings.LastIndex(file, "-")]
		var p Package
		err := datastore.Get(c, packageKey(c, name), &p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		details, err := decodeDetails(&p.Details)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(details.Readme) == 0 {
			fmt.Fprintf(w, "%v", p.Description)
		} else {
			// These \r's will show up as "^M" in the emacs buffer.
			// We don't want that, although hopefully package.el will
			// eventually fix this.
			fmt.Fprintf(w, "%v", strings.Replace(details.Readme, "\r", "", -1))
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

func requiredList(b *[]byte) string {
	details, err := decodeDetails(b)
	if err != nil || len(details.Required) == 0 {
		// TODO(ahyatt) Log an error here
		return "nil"
	}
	parts := make([]string, 0)
	for _, require := range details.Required {
		parts = append(parts, "(" + require.Name + " (" + 
			strings.Replace(require.Version, ".", " ", -1) + "))")
	}
	return "(" + strings.Join(parts, " ") + ")"
}

var templates = template.Must(template.ParseGlob("templates/*"))
var archiveContentsTemplate = template.Must(template.New("ArchiveContents").
Funcs(template.FuncMap{"versionList": versionList,
	"requiredList": requiredList}).
	Parse(archiveContentsElisp))

var archiveContentsElisp = `(1 {{range .}}
({{.Name}} . [{{versionList .LatestVersion}} {{requiredList .Details}} "{{.Description}}" single]){{end}})
`
