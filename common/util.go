/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.

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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/ini.v1"
)

// Sector size of disk
const SectorSize = 4096

var RootMount bool
var ForegroundMount bool

// IsDirectoryMounted is a utility function that returns true if the directory is already mounted using fuse
func IsDirectoryMounted(path string) bool {
	mntList, err := os.ReadFile("/etc/mtab")
	if err != nil {
		//fmt.Println("failed to read mount points : ", err.Error())
		return false
	}

	// removing trailing / from the path
	path = strings.TrimRight(path, "/")

	for _, line := range strings.Split(string(mntList), "\n") {
		if strings.TrimSpace(line) != "" {
			mntPoint := strings.Split(line, " ")[1]
			if path == mntPoint {
				// with earlier fuse driver ' fuse.' was searched in /etc/mtab
				// however with libfuse entry does not have that signature
				// if this path is already mounted using fuse then fail
				if strings.Contains(line, "fuse") {
					//fmt.Println(path, " is already mounted.")
					return true
				}
			}
		}
	}

	return false
}

// IsDirectoryEmpty is a utility function that returns true if the directory at that path is empty or not
func IsDirectoryEmpty(path string) bool {
	f, _ := os.Open(path)
	defer f.Close()

	_, err := f.Readdirnames(1)
	if err == io.EOF {
		return true
	}

	if err != nil && err.Error() == "invalid argument" {
		fmt.Println("Broken Mount : First Unmount ", path)
	}

	return false
}

// DirectoryExists is a utility function that returns true if the directory at that path exists and returns false if it does not exist.
func DirectoryExists(path string) bool {
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	} else if err != nil {
		return false
	}
	return true
}

// GetCurrentUser is a utility function that returns the UID and GID of the user that invokes the cloudfuse command.
func GetCurrentUser() (uint32, uint32, error) {
	var (
		currentUser      *user.User
		userUID, userGID uint64
	)

	currentUser, err := user.Current()
	if err != nil {
		return 0, 0, err
	}

	if runtime.GOOS == "windows" {
		r := regexp.MustCompile(`(?P<type>[A-Za-z]+)=[^\(]+\([^\)]+\) \([ug]id=(?P<id>[0-9]+)\)`)

		out, err := exec.Command(`C:\Program Files (x86)\WinFsp\bin\fsptool-x64.exe`, "id").Output()
		if err != nil {
			return 0, 0, fmt.Errorf("Is WinFSP installed? 'fsptool-x64.exe id' failed with error: %w", err)
		}

		idMap := make(map[string]string)
		for _, subMatches := range r.FindAllSubmatch(out, -1) {
			entityType := string(subMatches[r.SubexpIndex("type")])
			entityId := string(subMatches[r.SubexpIndex("id")])
			idMap[entityType] = entityId
		}
		// These keys come from fsptool: https://github.com/winfsp/winfsp/blob/master/src/fsptool/fsptool.c#L420
		// The "User" and "Owner" ID have been the same in my experience
		currentUser.Uid = idMap["User"]
		currentUser.Gid = idMap["Group"]
	}

	userUID, err = strconv.ParseUint(currentUser.Uid, 10, 32)
	if err != nil {
		return 0, 0, err
	}

	userGID, err = strconv.ParseUint(currentUser.Gid, 10, 32)
	if err != nil {
		return 0, 0, err
	}

	if currentUser.Name == "root" || userUID == 0 {
		RootMount = true
	} else {
		RootMount = false
	}

	return uint32(userUID), uint32(userGID), nil
}

// JoinUnixFilepath uses filepath.join to join a path and ensures that
// path only uses unix path delimiters.
func JoinUnixFilepath(elem ...string) string {
	return NormalizeObjectName(path.Join(elem...))
}

// normalizeObjectName : If file contains \\ in name replace it with ..
func NormalizeObjectName(name string) string {
	return strings.ReplaceAll(name, "\\", "/")
}

// Encrypt given data using the key provided
func EncryptData(plainData []byte, key string) ([]byte, error) {
	binaryKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode passphrase [%s]", err.Error())
	}

	block, err := aes.NewCipher(binaryKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plainData, nil)
	return ciphertext, nil
}

// Decrypt given data using the key provided
func DecryptData(cipherData []byte, key string) ([]byte, error) {
	binaryKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode passphrase [%s]", err.Error())
	}

	block, err := aes.NewCipher(binaryKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := cipherData[:gcm.NonceSize()]
	ciphertext := cipherData[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func GetCurrentDistro() string {
	cfg, err := ini.Load("/etc/os-release")
	if err != nil {
		return ""
	}

	distro := cfg.Section("").Key("PRETTY_NAME").String()
	return distro
}

type BitMap16 uint16

// IsSet : Check whether the given bit is set or not
func (bm BitMap16) IsSet(bit uint16) bool { return (bm & (1 << bit)) != 0 }

// Set : Set the given bit in bitmap
func (bm *BitMap16) Set(bit uint16) { *bm |= (1 << bit) }

// Clear : Clear the given bit from bitmap
func (bm *BitMap16) Clear(bit uint16) { *bm &= ^(1 << bit) }

// Reset : Reset the whole bitmap by setting it to 0
func (bm *BitMap16) Reset() { *bm = 0 }

type KeyedMutex struct {
	mutexes sync.Map // Zero value is empty and ready for use
}

func (m *KeyedMutex) GetLock(key string) *sync.Mutex {
	value, _ := m.mutexes.LoadOrStore(key, &sync.Mutex{})
	mtx := value.(*sync.Mutex)
	return mtx
}

// check if health monitor is enabled and blofuse stats monitor is not disabled
func MonitorCfs() bool {
	return EnableMonitoring && !CfsDisabled
}

// convert ~ to $HOME in path
func ExpandPath(path string) string {
	if path == "" {
		return path
	}

	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		path = JoinUnixFilepath(homeDir, path[2:])
	} else if strings.HasPrefix(path, "$HOME/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		path = JoinUnixFilepath(homeDir, path[6:])
	} else if strings.HasPrefix(path, "/$HOME/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		path = JoinUnixFilepath(homeDir, path[7:])
	}

	// If it is a drive letter don't add a trailing slash
	if IsDriveLetter(path) {
		return path
	}

	path = os.ExpandEnv(path)
	path, _ = filepath.Abs(path)
	path = JoinUnixFilepath(path)
	return path
}

// IsDriveLetter returns true if the path is a drive letter on Windows, such
// as 'D:' or 'f:'. Returns false otherwise.
func IsDriveLetter(path string) bool {
	pattern := `^[A-Za-z]:$`
	match, _ := regexp.MatchString(pattern, path)
	return match
}
