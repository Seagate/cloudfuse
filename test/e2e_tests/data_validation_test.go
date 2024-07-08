//go:build !unittest
// +build !unittest

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

package e2e_tests

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

var dataValidationMntPathPtr string
var dataValidationTempPathPtr string
var dataValidationAdlsPtr string
var dataValidationQuickTest string
var dataValidationStreamDirectTest string
var fileTestDistro string

var minBuff, medBuff, largeBuff, hugeBuff []byte

type dataValidationTestSuite struct {
	suite.Suite
	testMntPath   string
	testLocalPath string
	testCachePath string
	adlsTest      bool
}

func regDataValidationTestFlag(p *string, name string, value string, usage string) {
	if flag.Lookup(name) == nil {
		flag.StringVar(p, name, value, usage)
	}
}

func getDataValidationTestFlag(name string) string {
	return flag.Lookup(name).Value.(flag.Getter).Get().(string)
}

func initDataValidationFlags() {
	dataValidationMntPathPtr = getDataValidationTestFlag("mnt-path")
	dataValidationAdlsPtr = getDataValidationTestFlag("adls")
	dataValidationTempPathPtr = getDataValidationTestFlag("tmp-path")
	dataValidationQuickTest = getDataValidationTestFlag("quick-test")
	dataValidationStreamDirectTest = getDataValidationTestFlag("stream-direct-test")
	fileTestDistro = getDataValidationTestFlag("distro-name")
}

func getDataValidationTestDirName(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:n]
}

func (suite *dataValidationTestSuite) dataValidationTestCleanup(toRemove []string) {
	for _, path := range toRemove {
		err := os.RemoveAll(path)
		suite.NoError(err)
	}
}

func (suite *dataValidationTestSuite) copyToMountDir(localFilePath string, remoteFilePath string) {
	// copy to mounted directory
	suite.T().Helper()

	cpCmd := exec.Command("cp", localFilePath, remoteFilePath)
	cliOut, err := cpCmd.Output()
	if len(cliOut) != 0 {
		fmt.Println(string(cliOut))
	}
	suite.NoError(err)
}

func (suite *dataValidationTestSuite) validateData(localFilePath string, remoteFilePath string) {
	// compare the local and mounted files
	suite.T().Helper()

	diffCmd := exec.Command("diff", localFilePath, remoteFilePath)
	cliOut, err := diffCmd.Output()
	if len(cliOut) != 0 {
		fmt.Println(string(cliOut))
	}
	suite.Empty(cliOut)
	suite.NoError(err)
}

// -------------- Data Validation Tests -------------------

// Test correct overwrite of file using echo command
func (suite *dataValidationTestSuite) TestFileOverwriteWithEchoCommand() {

	if strings.Contains(strings.ToUpper(fileTestDistro), "UBUNTU-20.04") {
		fmt.Println("Skipping this test case for UBUNTU-20.04")
		return
	}

	remoteFilePath := filepath.Join(suite.testMntPath, "TESTFORECHO.txt")
	text := "Hello, this is a test."
	command := "echo \"" + text + "\" > " + remoteFilePath
	cmd := exec.Command("/bin/bash", "-c", command)
	_, err := cmd.Output()
	suite.NoError(err)

	data, err := os.ReadFile(remoteFilePath)
	suite.NoError(err)
	suite.Equal(string(data), text+"\n")

	newtext := "End of test."
	newcommand := "echo \"" + newtext + "\" > " + remoteFilePath
	newcmd := exec.Command("/bin/bash", "-c", newcommand)
	_, err = newcmd.Output()
	suite.NoError(err)

	data, err = os.ReadFile(remoteFilePath)
	suite.NoError(err)
	suite.Equal(string(data), newtext+"\n")
}

// data validation for small sized files
func (suite *dataValidationTestSuite) TestSmallFileData() {
	fileName := "small_data.txt"
	localFilePath := filepath.Join(suite.testLocalPath, fileName)
	remoteFilePath := filepath.Join(suite.testMntPath, fileName)

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, minBuff, 0777)
	suite.NoError(err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{suite.testCachePath})

	suite.validateData(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

// data validation for medium sized files
func (suite *dataValidationTestSuite) TestMediumFileData() {
	if strings.ToLower(dataValidationStreamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	fileName := "medium_data.txt"
	localFilePath := filepath.Join(suite.testLocalPath, fileName)
	remoteFilePath := filepath.Join(suite.testMntPath, fileName)

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, medBuff, 0777)
	suite.NoError(err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{suite.testCachePath})

	suite.validateData(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

// data validation for large sized files
func (suite *dataValidationTestSuite) TestLargeFileData() {
	if strings.ToLower(dataValidationStreamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	fileName := "large_data.txt"
	localFilePath := filepath.Join(suite.testLocalPath, fileName)
	remoteFilePath := filepath.Join(suite.testMntPath, fileName)

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, largeBuff, 0777)
	suite.NoError(err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{suite.testCachePath})

	suite.validateData(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

// negative test case for data validation where the local file is updated
func (suite *dataValidationTestSuite) TestDataValidationNegative() {
	fileName := "updated_data.txt"
	localFilePath := filepath.Join(suite.testLocalPath, fileName)
	remoteFilePath := filepath.Join(suite.testMntPath, fileName)

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, minBuff, 0777)
	suite.NoError(err)

	// copy local file to mounted directory
	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{suite.testCachePath})

	// update local file
	srcFile, err = os.OpenFile(localFilePath, os.O_APPEND|os.O_WRONLY, 0777)
	suite.NoError(err)
	_, err = srcFile.WriteString("Added text")
	srcFile.Close()
	suite.NoError(err)

	// compare local file and mounted files
	diffCmd := exec.Command("diff", localFilePath, remoteFilePath)
	cliOut, err := diffCmd.Output()
	fmt.Println("Negative test case where files should differ")
	fmt.Println(string(cliOut))
	suite.NotEmpty(cliOut)
	suite.Error(err)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

func validateMultipleFilesData(jobs <-chan int, results chan<- string, fileSize string, suite *dataValidationTestSuite) {
	for i := range jobs {
		fileName := fileSize + strconv.Itoa(i) + ".txt"
		localFilePath := filepath.Join(suite.testLocalPath, fileName)
		remoteFilePath := filepath.Join(suite.testMntPath, fileName)
		fmt.Println("Local file path: " + localFilePath)

		// create the file in local directory
		srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
		suite.NoError(err)
		srcFile.Close()

		// write to file in the local directory
		if fileSize == "huge" {
			err = os.WriteFile(localFilePath, hugeBuff, 0777)
		} else if fileSize == "large" {
			if strings.ToLower(dataValidationQuickTest) == "true" {
				err = os.WriteFile(localFilePath, hugeBuff, 0777)
			} else {
				err = os.WriteFile(localFilePath, largeBuff, 0777)
			}
		} else if fileSize == "medium" {
			err = os.WriteFile(localFilePath, medBuff, 0777)
		} else {
			err = os.WriteFile(localFilePath, minBuff, 0777)
		}
		suite.NoError(err)

		suite.copyToMountDir(localFilePath, remoteFilePath)
		suite.dataValidationTestCleanup([]string{filepath.Join(suite.testCachePath, fileName)})
		suite.validateData(localFilePath, remoteFilePath)

		suite.dataValidationTestCleanup([]string{localFilePath, filepath.Join(suite.testCachePath, fileName)})

		results <- remoteFilePath
	}
}

func createThreadPool(noOfFiles int, noOfWorkers int, fileSize string, suite *dataValidationTestSuite) {
	jobs := make(chan int, noOfFiles)
	results := make(chan string, noOfFiles)

	for i := 1; i <= noOfWorkers; i++ {
		go validateMultipleFilesData(jobs, results, fileSize, suite)
	}

	for i := 1; i <= noOfFiles; i++ {
		jobs <- i
	}
	close(jobs)

	for i := 1; i <= noOfFiles; i++ {
		filePath := <-results
		os.Remove(filePath)
	}
	close(results)

	suite.dataValidationTestCleanup([]string{suite.testCachePath})
}

func (suite *dataValidationTestSuite) TestMultipleSmallFiles() {
	noOfFiles := 16
	noOfWorkers := 4
	createThreadPool(noOfFiles, noOfWorkers, "small", suite)
}

func (suite *dataValidationTestSuite) TestMultipleMediumFiles() {
	if strings.ToLower(dataValidationStreamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	if strings.Contains(strings.ToUpper(fileTestDistro), "RHEL") {
		fmt.Println("Skipping this test case for RHEL")
		return
	}

	noOfFiles := 8
	noOfWorkers := 4
	createThreadPool(noOfFiles, noOfWorkers, "medium", suite)
}

func (suite *dataValidationTestSuite) TestMultipleLargeFiles() {
	if strings.ToLower(dataValidationStreamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	if strings.Contains(strings.ToUpper(fileTestDistro), "RHEL") {
		fmt.Println("Skipping this test case for RHEL")
		return
	}

	noOfFiles := 4
	noOfWorkers := 2
	createThreadPool(noOfFiles, noOfWorkers, "large", suite)
}

func (suite *dataValidationTestSuite) TestMultipleHugeFiles() {
	if strings.ToLower(dataValidationStreamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	if strings.ToLower(dataValidationQuickTest) == "true" {
		fmt.Println("Quick test is enabled. Skipping this test case")
		return
	}

	noOfFiles := 2
	noOfWorkers := 2
	createThreadPool(noOfFiles, noOfWorkers, "huge", suite)
}

// -------------- Main Method -------------------
func TestDataValidationTestSuite(t *testing.T) {
	initDataValidationFlags()
	fmt.Println("Distro Name: " + fileTestDistro)

	// Ignore data validation test on all distros other than UBN
	if strings.ToLower(dataValidationQuickTest) == "true" {
		fmt.Println("Skipping Data Validation test suite...")
		return
	}

	dataValidationTest := dataValidationTestSuite{}

	minBuff = make([]byte, 1024)
	medBuff = make([]byte, (10 * 1024 * 1024))
	largeBuff = make([]byte, (500 * 1024 * 1024))
	if strings.ToLower(dataValidationQuickTest) == "true" {
		hugeBuff = make([]byte, (100 * 1024 * 1024))
	} else {
		hugeBuff = make([]byte, (750 * 1024 * 1024))
	}

	// Generate random test dir name where our End to End test run is contained
	testDirName := getDataValidationTestDirName(10)

	// Create directory for testing the End to End test on mount path
	dataValidationTest.testMntPath = filepath.Join(dataValidationMntPathPtr, testDirName)
	fmt.Println(dataValidationTest.testMntPath)

	dataValidationTest.testLocalPath, _ = filepath.Abs(dataValidationMntPathPtr + "/..")
	fmt.Println(dataValidationTest.testLocalPath)

	dataValidationTest.testCachePath = filepath.Join(dataValidationTempPathPtr, testDirName)
	fmt.Println(dataValidationTest.testCachePath)

	if dataValidationAdlsPtr == "true" || dataValidationAdlsPtr == "True" {
		fmt.Println("ADLS Testing...")
		dataValidationTest.adlsTest = true
	} else {
		fmt.Println("BLOCK Blob Testing...")
	}

	// Sanity check in the off chance the same random name was generated twice and was still around somehow
	err := os.RemoveAll(dataValidationTest.testMntPath)
	if err != nil {
		fmt.Printf("TestDataValidationTestSuite : Could not cleanup mount dir before testing. Here's why: %v\n", err)
	}
	err = os.RemoveAll(dataValidationTest.testCachePath)
	if err != nil {
		fmt.Printf("TestDataValidationTestSuite : Could not cleanup cache dir before testing. Here's why: %v\n", err)
	}

	err = os.Mkdir(dataValidationTest.testMntPath, 0777)
	if err != nil {
		t.Error("Failed to create test directory")
	}
	rand.Read(minBuff)
	rand.Read(medBuff)
	rand.Read(largeBuff)
	rand.Read(hugeBuff)

	// Run the actual End to End test
	suite.Run(t, &dataValidationTest)

	//  Wipe out the test directory created for End to End test
	os.RemoveAll(dataValidationTest.testMntPath)
}

func init() {
	regDataValidationTestFlag(&dataValidationMntPathPtr, "mnt-path", "", "Mount Path of Container")
	regDataValidationTestFlag(&dataValidationAdlsPtr, "adls", "", "Account is ADLS or not")
	regDataValidationTestFlag(&dataValidationTempPathPtr, "tmp-path", "", "Cache dir path")
	regDataValidationTestFlag(&dataValidationQuickTest, "quick-test", "true", "Run quick tests")
	regDataValidationTestFlag(&dataValidationStreamDirectTest, "stream-direct-test", "false", "Run stream direct tests")
	regDataValidationTestFlag(&fileTestDistro, "distro-name", "", "Name of the distro")
}
