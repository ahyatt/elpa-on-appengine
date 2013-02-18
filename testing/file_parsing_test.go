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
	"bufio"
	"bytes"
	"fmt"
	"time"
	"strings"
	"testing"
	"archive/tar"
)

var validEquivalentPkgFiles = []string{
	`(define-package "sample-test" "0.1.2.3" "A sample package"
   '((req1 "1.0.0") (req2 "2.0.0") (req3 "3.0.0")))`,
	`(define-package "sample-test" "0.1.2.3" "A sample package"
   (quote ((req1 "1.0.0") (req2 "2.0.0") (req3 "3.0.0"))))`,
	`(DEFINE-PACKAGE "sample-test" "0.1.2.3" "A sample package"
   (QUOTE ((REQ1 "1.0.0") (REQ2 "2.0.0") (REQ3 "3.0.0"))))`,
}

func TestParsePackageVarsFromFile_empty(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(""))
	_, err := parsePackageVarsFromFile(reader)
	if err == nil {
		t.Fatal("Empty string should have returned an error")
	}
}

var sampleHeader string = `;;; sample-test.el --- A sample package
;;
;; Copyright (c) 2013 Andrew Hyatt
;;
;; Author: Andrew Hyatt <ahyatt@gmail.com>
;; Homepage: http://ignore.for.now
;; URL: http://also.ignored
;; Version: 0.1.2.3
;; Last-Updated: 19 Aug 2012
;; Keywords: fee, fi, fo, fum
;; Package-Requires: ((req1 "1.0.0") (req2 "2.0.0") (req3 "3.0.0"))
;;
;; Simplified BSD License
;;
;;; Commentary:
;;
;; This is the package commentary,
;; which spans multiple lines.
;;
;;; Code:
;;; Etc...
`

func TestParsePackageVarsFromFile_complete(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(sampleHeader))
	pkg, err := parsePackageVarsFromFile(reader)
	if err != nil {
		t.Fatal("Package populate should not have returned an error: ", err)
	}
	if pkg.Name != "sample-test" {
		t.Error("pkg.Name incorrect: ", pkg.Name)
	}
	if pkg.Description != "A sample package" {
		t.Error("pkg.Description incorrect: ", pkg.Description)
	}
	if pkg.LatestVersion != "0.1.2.3" {
		t.Error("pkg.Version incorrect: ", pkg.LatestVersion)
	}
	if pkg.Author != "Andrew Hyatt <ahyatt@gmail.com>" {
		t.Error("pkg.Author incorrect: ", pkg.Author)
	}
	details, err := decodeDetails(&pkg.Details)
	if len(details.Required) != 3 {
		t.Fatal("details.Required should have 3 elements, instead it has ", len(details.Required))
		// We'll just test the first & last element
		if details.Required[0].Name != "req1" || details.Required[0].Version != "1.0.0" ||
			details.Required[2].Name != "req3" || details.Required[3].Version != "3.0.0" {
			t.Fatal("details.Required incorrect:", details.Required)
		}
	}
	if details.Readme != "This is the package commentary,\nwhich spans multiple lines.\n" {
		t.Fatal("details.Readme incorrect: ", details.Readme)
	}
}

func WriteTarFile(t *testing.T, tw *tar.Writer, filename string, contents string) {
	if err := tw.WriteHeader(&tar.Header{
		Name:    filename,
		Size:    int64(len(contents)),
		ModTime: time.Now(),
	}); err != nil {
		t.Fatal("Could not create the header for testing")
	}
	fmt.Fprintf(tw, contents)
}

func TestParsePackageVarsFromTar_noDirectory(t *testing.T) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	WriteTarFile(t, tw, "foobar-pkg.el", validEquivalentPkgFiles[0])
	WriteTarFile(t, tw, "foobar.el", "test contents")
	tw.Close()
	_, err := parsePackageVarsFromTar(bufio.NewReader(buf))
	if err == nil {
		t.Fatal("Should have received an error due to not having a directory")
	}
}

func TestParsePackageVarsFromTar_twoDifferentDirectories(t *testing.T) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	WriteTarFile(t, tw, "foo-1.0/foobar-pkg.el", validEquivalentPkgFiles[0])
	WriteTarFile(t, tw, "foo-1.2/foobar.el", "test contents")
	tw.Close()
	_, err := parsePackageVarsFromTar(bufio.NewReader(buf))
	if err == nil {
		t.Fatal("Should have received an error from using two different top-level dirs")
	}
}

func assertValidPkgFile(t *testing.T, pkgFile string) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	WriteTarFile(t, tw, "sample-test-0.1.2.3/sample-test-pkg.el", pkgFile)
	WriteTarFile(t, tw, "sample-test-0.1.2.3/sample-test.el", "test contents")
	WriteTarFile(t, tw, "sample-test-0.1.2.3/README", "readme")
	tw.Close()
	pkg, err := parsePackageVarsFromTar(bufio.NewReader(buf))
	if err != nil {
		t.Fatal("No errors should be detected: ", err, "for pkg file", pkgFile)
	}
	if pkg == nil {
		t.Fatal("Package should not be nil for pkg file", pkgFile)
	}
	if pkg.Name != "sample-test" {
		t.Error("pkg.Name incorrect: ", pkg.Name, "for pkg file", pkgFile)
	}
	if pkg.Description != "A sample package" {
		t.Error("pkg.Description incorrect: ", pkg.Description, "for pkg file", pkgFile)
	}
	if pkg.LatestVersion != "0.1.2.3" {
		t.Error("pkg.Version incorrect: ", pkg.LatestVersion, "for pkg file", pkgFile)
	}
	details, err := decodeDetails(&pkg.Details)
	if details == nil {
		panic("Details is nil for pkg file" + pkgFile)
	}
	if len(details.Required) != 3 {
		t.Fatal("details.Required should have 3 elements, instead it has ", len(details.Required), "for pkg file", pkgFile)
		// We'll just test the first & last element
		if details.Required[0].Name != "req1" || details.Required[0].Version != "1.0.0" ||
			details.Required[2].Name != "req3" || details.Required[3].Version != "3.0.0" {
			t.Fatal("details.Required incorrect:", details.Required, "for pkg file", pkgFile)
		}
	}
	if details.Readme != "readme" {
		t.Fatal("Readme file should be 'readme', instead it is", details.Readme, "for pkg file", pkgFile)
	}
}

func TestParsePackageVarsFromTar_allValidForms(t *testing.T) {
	for _, pkgFile := range validEquivalentPkgFiles {
		assertValidPkgFile(t, pkgFile)
	}
}

func parsePackageDefinitionTester(pkg *Package, details *Details, def string) error {
	cin := make(chan int)
	cout := make(chan *Token)
	cerr := make(chan error)
	cdone := make(chan bool)
	go parseSimpleSexp(cin, cout, cdone)
	go readPackageDefinition(cout, cerr, pkg, details)
	for _, b := range []byte(def) {
		select {
		case err := <-cerr:
			return err
		default:
			cin <- int(b)
		}
	}
	cdone <- true
	return <-cerr
}

func TestParsePackageVarsFromTar_validMinimalPkgFile(t *testing.T) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	WriteTarFile(t, tw, "sample-test-0.1.2.3/sample-test-pkg.el",
		`(define-package "sample-test" "0.1.2.3")`)
	tw.Close()
	pkg, err := parsePackageVarsFromTar(bufio.NewReader(buf))
	if err != nil {
		t.Fatal("No errors should be detected: ", err)
	}
	if pkg == nil {
		t.Fatal("Package should not be nil")
	}
	if pkg.Name != "sample-test" {
		t.Error("pkg.Name incorrect: ", pkg.Name)
	}
	if pkg.LatestVersion != "0.1.2.3" {
		t.Error("pkg.Version incorrect: ", pkg.LatestVersion)
	}
	details, err := decodeDetails(&pkg.Details)
	if details == nil {
		panic("Details is nil!")
	}
	if len(details.Required) != 0 {
		t.Fatal("details.Required should have 0 elements, instead it has ", len(details.Required))
	}
}

func assertParsePackageFails(def string, pkg *Package, t *testing.T) {
	var details Details
	err := parsePackageDefinitionTester(pkg, &details, def)
	if err == nil {
		t.Fatal(fmt.Sprintf("Package definition '%s' should have failed",
			def))
	}
}

func TestBadPackageDefs(t *testing.T) {
	pkg := Package{Name: "foo", LatestVersion: "1.2.3"}
	// wrong name
	assertParsePackageFails(`(define-package "bar" "1.2.3" "A sample package"
   '((req1 "1.0.0") (req2 "2.0.0") (req3 "3.0.0")))`, &pkg, t)
	// wrong version
	assertParsePackageFails(`(define-package "foo" "3.2.1" "A sample package"
   '((req1 "1.0.0") (req2 "2.0.0") (req3 "3.0.0")))`, &pkg, t)
	// Malformed sexp
	assertParsePackageFails(`(define-package "foo" "1.2.3"`, &pkg, t)
	// No define-package
	assertParsePackageFails(`(+ 3 3)`, &pkg, t)
}
