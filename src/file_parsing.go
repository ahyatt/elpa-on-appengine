// Copyright 2013 Google Inc. All Rights Reserved.
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

// This file should have only non-appengine dependent code, so
// that tests can be run against it.

package elpa

import (
	"archive/tar"
	"bufio"
	"bytes"
	"errors"
	"encoding/gob"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
)

var elParamRE = regexp.MustCompile("^;; ([\\w\\-]+): (.*)")
var requiresRE = regexp.MustCompile("\\(([\\w\\-]+) \"([0-9\\.]+)\"\\)")
var nameDescriptionRE = regexp.MustCompile("^;;; ([\\w-\\-]+)\\.el --- (.*)")
var headingRe = regexp.MustCompile("^;;; (.*):")
var textLineRe = regexp.MustCompile("^;; (.*)")
var dirRe = regexp.MustCompile("^([\\w\\-]+)-([\\d\\.]+)")
var pkgFileNameRe = regexp.MustCompile("^([\\w\\-]+)-pkg.el")

func readPackageDefinition(cout chan *Token, cerr chan error, pkg *Package, details *Details) {
	tok := <-cout
	if tok.Type != OPEN_PAREN {
		cerr <- errors.New("Package definition must start with a open paren")
		return
	}
	tok = <- cout
	if tok.Type != SYMBOL && strings.ToLower(tok.StringVal) == "package-definition" {
		cerr <- errors.New("Package definition must start with '(package-defintion...'")
		return
	}
	tok = <- cout
	if tok.Type != STRING {
		cerr <- errors.New("Expected package name as first element in package definition")
		return
	}
	if tok.StringVal != pkg.Name {
		cerr <- errors.New(fmt.Sprintf("Package name in package definition (%s) didn't match directory name (%s)", tok.StringVal, pkg.Name))
		return
	}
	tok = <- cout
	if tok.Type != STRING {
		cerr <- errors.New("Expected version number as second element in package definition")
		return
	}
	if tok.StringVal != pkg.LatestVersion {
		cerr <- errors.New(fmt.Sprintf("Package version in package definition (%s) didn't match directory name (%s)", tok.StringVal, pkg.LatestVersion))
		return
	}
	tok = <- cout
	if tok.Type != CLOSE_PAREN {
		if tok.Type != STRING {
			cerr <- errors.New("Expected description as third element in package definition")
			return
		}
		pkg.Description = tok.StringVal
		tok = <- cout
		if tok.Type != CLOSE_PAREN {
			if tok.Type != SYMBOL || tok.StringVal != "nil" {
				if tok.Type != QUOTE {
					cerr <- errors.New("Unexpected tokens at the fourth element in package definition")
					return
				}
				tok = <- cout
				if tok.Type != OPEN_PAREN {
					cerr <- errors.New("Expected a list of lists at the fourth element in package definition")
					return
				}
				tok = <- cout
				for tok.Type == OPEN_PAREN {
					tok = <- cout
					if tok.Type != SYMBOL {
						cerr <- errors.New("Expected a symbol as the required package name")
						return
					}
					reqPackageName := tok.StringVal
					tok = <- cout
					if tok.Type != STRING {
						cerr <- errors.New("Expected a string as the required package version")
						return
					}
					reqPackageVersion := tok.StringVal
					details.Required = append(details.Required, PackageRef{Name: reqPackageName, Version: reqPackageVersion})
					tok = <- cout
					if tok.Type != CLOSE_PAREN {
						cerr <- errors.New("Required package should just be a 2-element list")
						return
					}
					tok = <- cout
				}
				if tok.Type != CLOSE_PAREN {
					cerr <- errors.New("Missing closing parenthesis for required versions ")
					return
				}
			}
			if tok.Type != CLOSE_PAREN {
				cerr <- errors.New("Missing closing parenthesis for required versions ")
				return
			}
		}
	}
	tok = <- cout
	if tok.Type != CLOSE_PAREN {
		cerr <- errors.New("Missing closing parenthesis for package definition")
	}
	cerr <- nil
}

func parsePackageDefinition(reader *tar.Reader, pkg *Package, details *Details) error {
	cin := make(chan int)
	cout := make(chan *Token)
	cerr := make(chan error)
	cdone := make(chan bool)
	go parseSimpleSexp(cin, cout, cdone)
	go readPackageDefinition(cout, cerr, pkg, details)
	bytes := make([]byte, 256)
	for {
		n, err := reader.Read(bytes);
		if err == io.EOF {
			cdone <- true
			break
		}
		if err != nil {
			cdone <- true
			return err
		}
		for _, b := range bytes[:n] {
			select {
			case err = <- cerr:
				return err
			default:
				cin <- int(b)
			}
		}
	}
	return <- cerr
}

func parsePackageVarsFromTar(reader *bufio.Reader) (*Package, error) {
	tr := tar.NewReader(reader)
	pkg := Package{}
	details := Details{}
	var dir *string = nil
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if dir == nil {
			name := filepath.Dir(hdr.Name)
			dirList := filepath.SplitList(name)
			if name == "." || len(dirList) == 0 {
				return nil, errors.New("Tar files must contain only files in a directory")
			}
			dir = &dirList[0]
			match := dirRe.FindStringSubmatch(*dir)
			if len(match) != 3 {
				return nil, errors.New("Directory must be '<package-name>-<version>/'")
			}
			pkg.Name = match[1]
			pkg.LatestVersion = match[2]
		} else {
			if (*dir != filepath.Dir(hdr.Name)) {
				return nil, errors.New("Tar files must only contain one top-level directory")
			}
		}
		if match := pkgFileNameRe.FindStringSubmatch(filepath.Base(hdr.Name)); len(match) > 0 && match[1] == pkg.Name {
			parsePackageDefinition(tr, &pkg, &details)
		}
		if filepath.Base(hdr.Name) == "README" {
			b, err := ioutil.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			details.Readme = string(b)
		}
	}
	bytes, err := encodeDetails(&details)
	if err != nil {
		return nil, err
	}
	pkg.Details = *bytes
	return &pkg, nil
}

func parsePackageVarsFromFile(reader *bufio.Reader) (*Package, error) {
	pkg := Package{}
	details := Details{}
	line, _ := reader.ReadString('\n')
	nameDescriptParts := nameDescriptionRE.FindStringSubmatch(line)
	if len(nameDescriptParts) == 3 {
		pkg.Name = nameDescriptParts[1]
		pkg.Description = strings.TrimSpace(nameDescriptParts[2])
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if parts := elParamRE.FindStringSubmatch(line); len(parts) > 0 {
			key := strings.ToLower(parts[1])
			value := parts[2]
			switch key {
			case "author":
				{
					pkg.Author = value
				}
			case "version":
				{
					pkg.LatestVersion = value
				}
			case "package-requires":
				{
					details.Required = make([]PackageRef, 0)
					for _, require := range requiresRE.FindAllStringSubmatch(value, -1) {
						if len(require) > 0 {
							details.Required = append(details.Required,
								PackageRef{
									Name:    require[1],
									Version: require[2],
								})
						}
					}
				}
			}
		}
		if parts := headingRe.FindStringSubmatch(line); len(parts) > 0 && parts[1] == "Commentary" {
			commentaryLines := make([]string, 0)
			for {
				commentaryLine, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				if len(headingRe.FindStringSubmatch(commentaryLine)) > 0 {
					break
				}
				commentaryParts := textLineRe.FindStringSubmatch(commentaryLine)
				if len(commentaryParts) > 0 {
					commentaryLines = append(commentaryLines, strings.TrimSpace(commentaryParts[1]))
				}
			}
			if len(commentaryLines) > 0 {
				details.Readme = strings.Join(commentaryLines, "\n") + "\n"
			}
		}
	}
	detailsPtr, err := encodeDetails(&details)
	pkg.Details = *detailsPtr
	if err != nil {
		return nil, err
	}

	if len(pkg.Name) == 0 || len(pkg.LatestVersion) == 0 || len(pkg.Description) == 0 {
		return nil, errors.New(fmt.Sprintf("Required attributes (name, version, or description) were missing.  Here's what we got: %#v", pkg))
	}
	return &pkg, nil
}

func encodeDetails(details *Details) (*[]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(details)
	if err != nil {
		return nil, err
	}
	b := make([]byte, buf.Len())
	_, err = buf.Read(b)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func decodeDetails(b *[]byte) (*Details, error) {
	var buf bytes.Buffer
	buf.Write(*b)
	decoder := gob.NewDecoder(&buf)
	var details Details
	err := decoder.Decode(&details)
	if err != nil {
		return nil, err
	}
	return &details, nil
}
