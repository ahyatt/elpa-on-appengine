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

// This file has data definitions that are not-appengine specific, so
// that tests can be run against parts of the code that manipulate
// these structs.

package elpa

type Package struct {
	Name          string       `datastore:name`
	Description   string       `datastore:description,noindex`
	LatestVersion string       `datastore:contentid,noindex`
	Author        string       `datastore:author`
	Details       []byte       `datastore:requires`
}

type Details struct {
	Readme  string
	Required []PackageRef
}

type PackageRef struct {
	Name    string
	Version string
}

