/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates

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

package common

import (
	"strings"
	"testing"
)

// FuzzNormalizeObjectName tests that NormalizeObjectName consistently converts
// backslashes to forward slashes without introducing new characters or panicking.
func FuzzNormalizeObjectName(f *testing.F) {
	f.Add("normal/path")
	f.Add("..\\..\\etc\\passwd")
	f.Add("path\\with\\backslash")
	f.Add("")
	f.Add("/")
	f.Add("C:\\Windows\\System32")
	f.Add("\\\\network\\share")
	f.Add("foo/bar\\baz/qux")
	f.Add(strings.Repeat("a", 1000))
	f.Add("path\x00with\x00nulls")
	f.Add("特殊字符\\路径")

	f.Fuzz(func(t *testing.T, input string) {
		result := NormalizeObjectName(input)

		if strings.Contains(result, "\\") {
			t.Errorf("result contains backslash: %q -> %q", input, result)
		}

		if len(result) != len(input) {
			t.Errorf("result length changed: input=%d, result=%d", len(input), len(result))
		}

		doubleNormalized := NormalizeObjectName(result)
		if doubleNormalized != result {
			t.Errorf("not idempotent: %q -> %q -> %q", input, result, doubleNormalized)
		}

		inputWithoutBackslash := strings.ReplaceAll(input, "\\", "/")
		if result != inputWithoutBackslash {
			t.Errorf("unexpected transformation: expected %q, got %q", inputWithoutBackslash, result)
		}
	})
}

// FuzzJoinUnixFilepath tests that JoinUnixFilepath produces valid Unix-style paths
// without backslashes and handles various input combinations safely.
func FuzzJoinUnixFilepath(f *testing.F) {
	f.Add("base", "file")
	f.Add("/root", "../etc/passwd")
	f.Add("", "file")
	f.Add("/", "")
	f.Add("foo/bar", "baz/qux")
	f.Add("C:\\Windows", "System32")
	f.Add("path", "..\\..\\etc\\passwd")
	f.Add("", "")
	f.Add("a", "b")

	f.Fuzz(func(t *testing.T, part1, part2 string) {
		result := JoinUnixFilepath(part1, part2)

		if strings.Contains(result, "\\") {
			t.Errorf("result contains backslash: JoinUnixFilepath(%q, %q) = %q", part1, part2, result)
		}
	})
}

