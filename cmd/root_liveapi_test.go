//go:build liveapi

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package cmd

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Marker prefixes are parsed by the weekly canary workflow.
const (
	canaryTransientPrefix = "CANARY_TRANSIENT"
	canaryAPIBreakPrefix  = "CANARY_API_BREAK"
)

func TestGitHubReleaseAPICanary(t *testing.T) {
	prevPackage := opt.Package
	t.Cleanup(func() {
		opt.Package = prevPackage
	})

	// Select a package that should exist for each platform so getRelease validates
	// the same asset-selection behavior used by production update flows.
	switch runtime.GOOS {
	case "windows":
		opt.Package = "exe"
	default:
		opt.Package = "rpm"
	}

	release, err := getRelease(context.Background(), "")
	if err != nil {
		if isTransientCanaryError(err) {
			t.Fatalf("%s: %v", canaryTransientPrefix, err)
		}
		t.Fatalf("%s: getRelease failed: %v", canaryAPIBreakPrefix, err)
	}

	if release == nil {
		t.Fatalf("%s: getRelease returned nil release", canaryAPIBreakPrefix)
	}

	if release.Version == "" {
		t.Fatalf("%s: empty version from release API", canaryAPIBreakPrefix)
	}

	if _, err := strconv.Atoi(strings.SplitN(release.Version, ".", 2)[0]); err != nil {
		t.Fatalf("%s: invalid version format %q", canaryAPIBreakPrefix, release.Version)
	}

	if release.AssetName == "" || release.AssetURL == "" {
		t.Fatalf("%s: empty asset metadata (name=%q url=%q)", canaryAPIBreakPrefix, release.AssetName, release.AssetURL)
	}
}

func isTransientCanaryError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "401") ||
		strings.Contains(msg, "403") ||
		strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "temporary") ||
		strings.Contains(msg, "tls handshake") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "service unavailable")
}