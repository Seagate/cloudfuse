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
	"bytes"
	"testing"

	"github.com/awnumar/memguard"
)

// FuzzEncryptDecryptRoundTrip tests that encrypt followed by decrypt returns original data
func FuzzEncryptDecryptRoundTrip(f *testing.F) {
	f.Add([]byte("hello world"))
	f.Add([]byte(""))
	f.Add([]byte("a"))
	f.Add(make([]byte, 1024))
	f.Add([]byte("special chars: !@#$%^&*()"))
	f.Add([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE})

	// Use a fixed, valid base64-encoded 64-byte key
	fixedKey := make([]byte, 64)
	for i := range fixedKey {
		fixedKey[i] = byte(i)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		enclave := memguard.NewEnclave(fixedKey)

		encrypted, err := EncryptData(data, enclave)
		if err != nil {
			return
		}

		decrypted, err := DecryptData(encrypted, enclave)
		if err != nil {
			t.Errorf("decryption failed after successful encryption: %v", err)
			return
		}

		if !bytes.Equal(data, decrypted) {
			t.Errorf("round-trip failed: got %v, want %v", decrypted, data)
		}
	})
}

// FuzzDecryptMalformed tests decryption with malformed ciphertext
func FuzzDecryptMalformed(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0x00, 0x00})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	f.Add(make([]byte, 100))
	f.Add([]byte("not encrypted data"))
	f.Add([]byte{0x10, 0x00}) // Claims 16 byte salt but no data
	f.Add([]byte{0xFF, 0xFF}) // Huge salt length

	// Use a fixed, valid 64-byte key
	fixedKey := make([]byte, 64)
	for i := range fixedKey {
		fixedKey[i] = byte(i)
	}

	f.Fuzz(func(t *testing.T, ciphertext []byte) {
		enclave := memguard.NewEnclave(fixedKey)

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("DecryptData panicked on input %v: %v", ciphertext, r)
			}
		}()

		_, _ = DecryptData(ciphertext, enclave)
	})
}
