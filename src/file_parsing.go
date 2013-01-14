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
	"bufio"
	"bytes"
	"errors"
	"encoding/gob"
	"fmt"
	"regexp"
	"strings"
)

var elParamRE = regexp.MustCompile("^;; ([\\w\\-]+): (.*)")
var requiresRE = regexp.MustCompile("\\(([\\w\\-]+) \"([0-9\\.]+)\"\\)")
var nameDescriptionRE = regexp.MustCompile("^;;; ([\\w-\\-]+)\\.el --- ([\\w\\s]+)")
var headingRe = regexp.MustCompile("^;;; (.*):")
var textLineRe = regexp.MustCompile("^;; (.*)")

func parsePackageVarsFromFile(reader *bufio.Reader) (*Package, error) {
	pkg := Package{}
	details := Details{}
	line, _ := reader.ReadString('\n')
	fmt.Println("Read line: ", line)
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