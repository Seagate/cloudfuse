/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.

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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
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
	"sync/atomic"

	"github.com/awnumar/memguard"
	"gopkg.in/ini.v1"
)

// Sector size of disk
const SectorSize = 4096

var RootMount bool
var ForegroundMount bool
var IsStream bool

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

func IsMountActive(path string) (bool, error) {
	// Get the process details for this path using ps -aux
	var out bytes.Buffer
	cmd := exec.Command("pidof", "cloudfuse")
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		if err.Error() == "exit status 1" {
			return false, nil
		} else {
			return true, fmt.Errorf("failed to get pid of cloudfuse [%v]", err.Error())
		}
	}

	// out contains the list of pids of the processes that are running
	pidString := strings.ReplaceAll(out.String(), "\n", " ")
	pids := strings.Split(pidString, " ")
	for _, pid := range pids {
		// Get the mount path for this pid
		// For this we need to check the command line arguments given to this command
		// If the path is same then we need to return true
		if pid == "" {
			continue
		}

		cmd = exec.Command("ps", "-o", "args=", "-p", pid)
		out.Reset()
		cmd.Stdout = &out

		err := cmd.Run()
		if err != nil {
			return true, fmt.Errorf(
				"failed to get command line arguments for pid %s [%v]",
				pid,
				err.Error(),
			)
		}

		if strings.Contains(out.String(), path) {
			return true, nil
		}
	}

	return false, nil
}

// IsDirectoryEmpty is a utility function that returns true if the directory at that path is empty or not
func IsDirectoryEmpty(path string) bool {
	if !DirectoryExists(path) {
		// Directory does not exists so safe to assume its empty
		return true
	}

	f, _ := os.Open(path)
	defer f.Close()

	_, err := f.Readdirnames(1)
	// If there is nothing in the directory then it is empty
	return err == io.EOF
}

func TempCacheCleanup(path string) error {
	if !IsDirectoryEmpty(path) {
		// List the first level children of the directory
		dirents, err := os.ReadDir(path)
		if err != nil {
			// Failed to list, return back error
			return fmt.Errorf("failed to list directory contents : %s", err.Error())
		}

		// Delete all first level children with their hierarchy
		for _, entry := range dirents {
			os.RemoveAll(filepath.Join(path, entry.Name()))
		}
	}

	return nil
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
			return 0, 0, fmt.Errorf(
				"is WinFSP installed? 'fsptool-x64.exe id' failed with error: %w",
				err,
			)
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
func EncryptData(plainData []byte, key *memguard.Enclave) ([]byte, error) {
	if key == nil {
		return nil, errors.New("provided passphrase key is empty")
	}

	secretKey, err := key.Open()
	if err != nil || secretKey == nil {
		return nil, errors.New("unable to decrypt passphrase key")
	}
	defer secretKey.Destroy()

	// A base64 encode of a key of length 32 will be at maximum a length of 44 bytes
	if len(secretKey.Bytes()) > 44 {
		return nil, errors.New("provided decoded base64 key is longer than 32 bytes. Decoded key " +
			"length shall be 16 (AES-128), 24 (AES-192), or 32 (AES-256) bytes in length")
	}

	decodedKey := make([]byte, 32) // Valid key can't be longer than 32 bytes
	_, err = base64.StdEncoding.Decode(decodedKey, secretKey.Bytes())
	if err != nil {
		return nil, err
	}
	decodedKey = bytes.Trim(decodedKey, "\x00") // trim any null bytes if key is 16, or 24 bytes

	block, err := aes.NewCipher(decodedKey)
	if err != nil {
		return nil, err
	}
	clear(decodedKey)

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
func DecryptData(cipherData []byte, key *memguard.Enclave) ([]byte, error) {
	if key == nil {
		return nil, errors.New("provided passphrase key is empty")
	}

	secretKey, err := key.Open()
	if err != nil || secretKey == nil {
		return nil, errors.New("unable to decrypt passphrase key")
	}
	defer secretKey.Destroy()

	// A base64 encode of a key of length 32 will be at maximum a length of 44 bytes
	if len(secretKey.Bytes()) > 44 {
		return nil, errors.New("provided decoded base64 key is longer than 32 bytes. Decoded key " +
			"length shall be 16 (AES-128), 24 (AES-192), or 32 (AES-256) bytes in length")
	}

	decodedKey := make([]byte, 32) // Valid key can't be longer than 32 bytes
	_, err = base64.StdEncoding.Decode(decodedKey, secretKey.Bytes())
	if err != nil {
		return nil, err
	}
	decodedKey = bytes.Trim(decodedKey, "\x00") // trim any null bytes if key is 16, or 24 bytes

	block, err := aes.NewCipher(decodedKey)
	if err != nil {
		return nil, err
	}
	clear(decodedKey)

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

type BitMap16 struct {
	val atomic.Uint32
}

func (bm *BitMap16) IsSet(bit uint16) bool {
	if bit >= 16 {
		return false
	}
	return (bm.val.Load() & (1 << bit)) != 0
}

func (bm *BitMap16) Set(bit uint16) {
	if bit >= 16 {
		return
	}
	mask := uint32(1 << bit)
	for {
		old := bm.val.Load()
		newVal := old | mask
		if newVal == old {
			return
		}
		if bm.val.CompareAndSwap(old, newVal) {
			return
		}
	}
}

func (bm *BitMap16) Clear(bit uint16) {
	if bit >= 16 {
		return
	}
	mask := uint32(1 << bit)
	for {
		old := bm.val.Load()
		newVal := old &^ mask
		if newVal == old {
			return
		}
		if bm.val.CompareAndSwap(old, newVal) {
			return
		}
	}
}

func (bm *BitMap16) Reset() {
	bm.val.Store(0)
}

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

func CreateDefaultDirectory() error {
	dir, err := os.Stat(ExpandPath(DefaultWorkDir))
	if err == nil && !dir.IsDir() {
		return err
	}

	if err != nil && os.IsNotExist(err) {
		// create the default work dir
		if err = os.MkdirAll(ExpandPath(DefaultWorkDir), 0755); err != nil {
			return err
		}
	}
	return nil
}

type WriteToFileOptions struct {
	Flags      int
	Permission os.FileMode
}

func WriteToFile(filename string, data string, options WriteToFileOptions) error {
	// Open the file with the provided flags, create it if it doesn't exist
	//check if options.Permission is 0 if so then assign 0644
	if options.Permission == 0 {
		options.Permission = 0644
	}
	file, err := os.OpenFile(filename, options.Flags|os.O_CREATE|os.O_WRONLY, options.Permission)
	if err != nil {
		return fmt.Errorf("error opening file: [%s]", err.Error())
	}
	defer file.Close() // Ensure the file is closed when we're done

	// Write the data content to the file
	if _, err := file.WriteString(data); err != nil {
		return fmt.Errorf("error writing to file [%s]", err.Error())
	}

	return nil
}
