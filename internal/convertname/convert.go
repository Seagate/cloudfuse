/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates

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

package convertname

// Map of characters and their similar looking unicode.
var cloudToFileMap = map[rune]rune{
	'"': '＂',
	'*': '＊',
	':': '：',
	'<': '＜',
	'>': '＞',
	'?': '？',
	'|': '｜',
}

var fileToCloudMap = reverseMap(cloudToFileMap)

func reverseMap(inMap map[rune]rune) map[rune]rune {
	var reverseMap = make(map[rune]rune)
	for k, v := range inMap {
		reverseMap[v] = k
	}
	return reverseMap
}

func replaceWithMap(s string, m map[rune]rune) string {
	runes := []rune(s)
	for i, r := range runes {
		if val, ok := m[r]; ok {
			runes[i] = val
		}
	}
	return string(runes)
}

// WindowsFileToCloud converts a filename on Windows that includes special unicode
// characters such as ＂＊：＜＞？｜ to the original characters "*:<>?|.
func WindowsFileToCloud(filename string) string {
	return replaceWithMap(filename, fileToCloudMap)
}

// WindowsCloudToFile converts an object name to a filename that converts the characters
// "*:<>?| to the similar unicode characters ＂＊：＜＞？｜.
func WindowsCloudToFile(cloudname string) string {
	return replaceWithMap(cloudname, cloudToFileMap)
}
