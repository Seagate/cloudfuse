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

package common

import "strconv"

func Uint64ToInt(v uint64) (int, bool) {
	if strconv.IntSize == 32 {
		if v > uint64(^uint32(0)>>1) {
			return 0, false
		}
		return int(v), true
	}

	if v > uint64(^uint64(0)>>1) {
		return 0, false
	}

	return int(v), true
}

func IntToUint32(v int) (uint32, bool) {
	if v < 0 || uint64(v) > uint64(^uint32(0)) {
		return 0, false
	}
	return uint32(v), true
}

func Uint64ToUint32(v uint64) (uint32, bool) {
	if v > uint64(^uint32(0)) {
		return 0, false
	}
	return uint32(v), true
}

func Int64ToUint64(v int64) (uint64, bool) {
	if v < 0 {
		return 0, false
	}
	return uint64(v), true
}

func Uint64ToInt64(v uint64) (int64, bool) {
	if v > uint64(^uint64(0)>>1) {
		return 0, false
	}
	return int64(v), true
}

func UintToUint32(v uint) (uint32, bool) {
	if uint64(v) > uint64(^uint32(0)) {
		return 0, false
	}
	return uint32(v), true
}

func IntToUint64(v int) (uint64, bool) {
	if v < 0 {
		return 0, false
	}
	return uint64(v), true
}

func IntToUint16(v int) (uint16, bool) {
	if v < 0 || v > int(^uint16(0)) {
		return 0, false
	}
	return uint16(v), true
}

func IntToInt32(v int) (int32, bool) {
	if v < -1<<31 || v > 1<<31-1 {
		return 0, false
	}
	return int32(v), true
}

func Int64ToInt32(v int64) (int32, bool) {
	if v < -1<<31 || v > 1<<31-1 {
		return 0, false
	}
	return int32(v), true
}

func RuneToByte(v rune) (byte, bool) {
	if v < 0 || v > 255 {
		return 0, false
	}
	return byte(v), true
}
