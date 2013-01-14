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
	"strings"
	"testing"
)

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