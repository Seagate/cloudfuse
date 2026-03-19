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

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc64"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/awnumar/memguard"
	"github.com/petermattis/goid"
	"gopkg.in/ini.v1"
)

// Sector size of disk
const SectorSize = 4096
const uint16Size = 2

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

	for line := range strings.SplitSeq(string(mntList), "\n") {
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
	myPid := strconv.Itoa(os.Getpid())
	for _, pid := range pids {
		// Get the mount path for this pid
		// For this we need to check the command line arguments given to this command
		// If the path is same then we need to return true
		if pid == "" || pid == myPid {
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
func EncryptData(plainData []byte, password *memguard.Enclave) ([]byte, error) {
	if password == nil {
		return nil, errors.New("provided password is empty")
	}

	secretKey, err := password.Open()
	if err != nil || secretKey == nil {
		return nil, errors.New("unable to decrypt password")
	}
	defer secretKey.Destroy()

	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("unable to generate random salt with error: %w", err)
	}
	key := deriveKey(secretKey.Bytes(), salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	clear(key)

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plainData, nil)

	// Write out encrypted file with length of salt, salt, length of nonce, nonce, and the ciphertext
	outputBuffer := new(bytes.Buffer)

	err = binary.Write(outputBuffer, binary.LittleEndian, SaltLength)
	if err != nil {
		return nil, err
	}
	outputBuffer.Write(salt)

	err = binary.Write(outputBuffer, binary.LittleEndian, uint16(gcm.NonceSize()))
	if err != nil {
		return nil, err
	}
	outputBuffer.Write(nonce)

	outputBuffer.Write(ciphertext)

	return outputBuffer.Bytes(), nil
}

// DecryptData decrypts the given data using the provided key.
func DecryptData(cipherData []byte, password *memguard.Enclave) ([]byte, error) {
	if password == nil {
		return nil, errors.New("provided password is empty")
	}

	secretKey, err := password.Open()
	if err != nil || secretKey == nil {
		return nil, errors.New("unable to decrypt password")
	}
	defer secretKey.Destroy()

	salt, err := extractSalt(cipherData)
	if err != nil {
		return nil, err
	}

	key := deriveKey(secretKey.Bytes(), salt)
	defer clear(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce, ciphertext, err := extractNonceAndCiphertext(cipherData)
	if err != nil {
		return nil, err
	}

	// Validate nonce length before passing to GCM to prevent panic
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf(
			"invalid nonce length: got %d, expected %d",
			len(nonce),
			gcm.NonceSize(),
		)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func extractSalt(cipherData []byte) ([]byte, error) {
	// Check minimum length for salt length field
	if len(cipherData) < int(uint16Size) {
		return nil, errors.New("cipher data too short to contain salt length")
	}
	saltLength := int(binary.LittleEndian.Uint16(cipherData[:uint16Size]))
	if len(cipherData) < uint16Size+saltLength {
		return nil, errors.New("invalid data length")
	}
	return cipherData[uint16Size : uint16Size+saltLength], nil
}

func extractNonceAndCiphertext(cipherData []byte) ([]byte, []byte, error) {
	// Check minimum length for salt length field
	if len(cipherData) < int(uint16Size) {
		return nil, nil, errors.New("cipher data too short to contain salt length")
	}
	saltLength := int(binary.LittleEndian.Uint16(cipherData[:uint16Size]))
	offset := uint16Size + saltLength

	// Check if data is long enough to contain nonce length field
	if len(cipherData) < offset+uint16Size {
		return nil, nil, errors.New("cipher data too short to contain nonce length")
	}
	nonceLength := int(binary.LittleEndian.Uint16(cipherData[offset : offset+uint16Size]))
	offset += uint16Size

	if len(cipherData) < offset+nonceLength {
		return nil, nil, errors.New("invalid data length")
	}

	nonce := cipherData[offset : offset+nonceLength]
	ciphertext := cipherData[offset+nonceLength:]

	return nonce, ciphertext, nil
}

func GetCurrentDistro() string {
	cfg, err := ini.Load("/etc/os-release")
	if err != nil {
		return ""
	}

	distro := cfg.Section("").Key("PRETTY_NAME").String()
	return distro
}

// ThreadSafe Bitmap Implementation
type BitMap64 uint64

// IsSet : Check whether the given bit is set or not
func (bm *BitMap64) IsSet(bit uint64) bool {
	return (atomic.LoadUint64((*uint64)(bm)) & (1 << bit)) != 0
}

// Set : Set the given bit in bitmap
// Return true if the bit was not set and was set by this call, false if the bit was already set.
func (bm *BitMap64) Set(bit uint64) bool {
	for {
		loaded := atomic.LoadUint64((*uint64)(bm))
		if (loaded & (1 << bit)) != 0 {
			// Bit already set.
			return false
		}
		newValue := loaded | (1 << bit)
		if atomic.CompareAndSwapUint64((*uint64)(bm), loaded, newValue) {
			// Bit was set successfully.
			return true
		}
	}
}

// Clear : Clear the given bit from bitmap
// Return true if the bit is set and cleared by this call, false if the bit was already cleared.
func (bm *BitMap64) Clear(bit uint64) bool {
	for {
		loaded := atomic.LoadUint64((*uint64)(bm))
		if (loaded & (1 << bit)) == 0 {
			// Bit already cleared.
			return false
		}
		newValue := loaded &^ (1 << bit)
		if atomic.CompareAndSwapUint64((*uint64)(bm), loaded, newValue) {
			// Bit was cleared successfully.
			return true
		}
	}
}

// Reset : Reset the whole bitmap by setting it to 0
// Return true if the bitmap is cleared by this call, false if it was already cleared.
func (bm *BitMap64) Reset() bool {
	for {
		loaded := atomic.LoadUint64((*uint64)(bm))
		if loaded == 0 {
			// Bitmap already cleared.
			return false
		}
		if atomic.CompareAndSwapUint64((*uint64)(bm), loaded, 0) {
			// Bitmap was cleared successfully.
			return true
		}
	}
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

	path = os.Expand(path, func(key string) string {
		if azureSpecialContainers[key] {
			return "$" + key // Keep it as is
		}
		return os.Getenv(key) // Expand normally
	})

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

func GetCRC64(data []byte, length int) []byte {
	// Create a CRC64 hash using the ECMA polynomial
	crc64Table := crc64.MakeTable(crc64.ECMA)
	checksum := crc64.Checksum(data[:length], crc64Table)

	checksumBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(checksumBytes, checksum)

	return checksumBytes
}

func SanitizeName(name string) string {
	replacer := strings.NewReplacer(
		"\\",
		"_",
		"/",
		"_",
		":",
		"_",
		"*",
		"_",
		"?",
		"_",
		"\"",
		"_",
		"<",
		"_",
		">",
		"_",
		"|",
		"_",
	)
	return replacer.Replace(name)
}

func GetMD5(fi *os.File) ([]byte, error) {
	hasher := md5.New()
	_, err := io.Copy(hasher, fi)

	if err != nil {
		return nil, fmt.Errorf("failed to generate md5 [%s]", err.Error())
	}

	return hasher.Sum(nil), nil
}

func ComponentInPipeline(pipeline []string, component string) bool {
	return slices.Contains(pipeline, component)
}

func ValidatePipeline(pipeline []string) error {
	// file-cache, block-cache and xload are mutually exclusive
	if ComponentInPipeline(pipeline, "file_cache") &&
		ComponentInPipeline(pipeline, "block_cache") {
		return fmt.Errorf("mount: file-cache and block-cache cannot be used together")
	}

	if ComponentInPipeline(pipeline, "file_cache") &&
		ComponentInPipeline(pipeline, "xload") {
		return fmt.Errorf("mount: file-cache and xload cannot be used together")
	}

	if ComponentInPipeline(pipeline, "block_cache") &&
		ComponentInPipeline(pipeline, "xload") {
		return fmt.Errorf("mount: block-cache and xload cannot be used together")
	}

	return nil
}

func UpdatePipeline(pipeline []string, component string) []string {
	if ComponentInPipeline(pipeline, component) {
		return pipeline
	}

	if component == "xload" {
		for i, comp := range pipeline {
			if comp == "file_cache" || comp == "block_cache" {
				pipeline[i] = component
				return pipeline
			}
		}
	}

	if component == "block_cache" {
		for i, comp := range pipeline {
			if comp == "file_cache" || comp == "xload" {
				pipeline[i] = component
				return pipeline
			}
		}
	}

	return pipeline
}

var openFlagNames = []struct {
	flag int
	name string
}{
	{os.O_RDONLY, "O_RDONLY"},
	{os.O_WRONLY, "O_WRONLY"},
	{os.O_RDWR, "O_RDWR"},
	{os.O_APPEND, "O_APPEND"},
	{os.O_CREATE, "O_CREATE"},
	{os.O_EXCL, "O_EXCL"},
	{os.O_SYNC, "O_SYNC"},
	{os.O_TRUNC, "O_TRUNC"},
}

func PrettyOpenFlags(f int) string {
	// Access mode is mutually exclusive, so handle separately
	access := f & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR)

	out := []string{}
	switch access {
	case os.O_RDONLY:
		out = append(out, "O_RDONLY")
	case os.O_WRONLY:
		out = append(out, "O_WRONLY")
	case os.O_RDWR:
		out = append(out, "O_RDWR")
	}

	// Check remaining flags
	for _, item := range openFlagNames {
		if item.flag == os.O_RDONLY || item.flag == os.O_WRONLY || item.flag == os.O_RDWR {
			continue // skip access flags already handled
		}
		if f&item.flag != 0 {
			out = append(out, item.name)
		}
	}

	return fmt.Sprintf("[%s]", strings.Join(out, " | "))
}

// GetGoroutineID returns the goroutine id of the current goroutine.
// It uses the goid package to retrieve the goroutine id which fetches it
// from the GO internal runtime data structures, instead of making expensive
// runtime.Stack calls.
func GetGoroutineID() uint64 {
	return (uint64)(goid.Get())
}
