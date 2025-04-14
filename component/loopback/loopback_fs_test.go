/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
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

package loopback

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const testPath = "/tmp/cloudfuselfstests"
const dirOne = "one"
const dirTwo = "two"
const dirEmpty = "empty"
const fileHello = "hello.txt"
const fileEmpty = "empty.txt"
const fileQuotes = "one/quotes.txt"
const fileLorem = "one/lorem.txt"

const quotesText = `
The Future belongs to those who believe in the beauty of their dreams
	- Eleanor Roosevelt
`
const loremText = `
Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Et molestie ac feugiat sed lectus vestibulum mattis ullamcorper. Elit scelerisque mauris pellentesque pulvinar. Vestibulum lectus mauris ultrices eros in cursus. Nec feugiat in fermentum posuere urna nec tincidunt praesent semper. Proin nibh nisl condimentum id. Diam vel quam elementum pulvinar. Elit at imperdiet dui accumsan sit. Turpis massa sed elementum tempus egestas sed sed risus. Rhoncus urna neque viverra justo nec ultrices. Aliquet eget sit amet tellus cras adipiscing enim. Eu facilisis sed odio morbi quis commodo odio aenean. Sit amet tellus cras adipiscing enim.

Nunc sed blandit libero volutpat sed cras ornare arcu dui. Tempor commodo ullamcorper a lacus vestibulum sed arcu non odio. Quam nulla porttitor massa id neque aliquam. Nullam ac tortor vitae purus faucibus ornare. Sit amet luctus venenatis lectus. Dignissim sodales ut eu sem integer vitae. Senectus et netus et malesuada. Amet consectetur adipiscing elit duis tristique. Id leo in vitae turpis. Lectus magna fringilla urna porttitor rhoncus dolor purus non. Lectus quam id leo in vitae turpis massa sed. Sed vulputate mi sit amet mauris commodo. Sem nulla pharetra diam sit amet nisl suscipit. Vulputate odio ut enim blandit volutpat. Pharetra sit amet aliquam id diam maecenas ultricies mi eget. Ipsum suspendisse ultrices gravida dictum fusce ut placerat orci nulla. Porttitor massa id neque aliquam vestibulum morbi blandit.

Semper quis lectus nulla at volutpat. Tellus rutrum tellus pellentesque eu tincidunt tortor aliquam. Nunc scelerisque viverra mauris in aliquam sem fringilla. Tincidunt dui ut ornare lectus sit amet. Pharetra magna ac placerat vestibulum lectus. Amet consectetur adipiscing elit duis tristique sollicitudin nibh sit. Augue eget arcu dictum varius duis at. Arcu ac tortor dignissim convallis aenean et tortor at. Mauris cursus mattis molestie a. Duis convallis convallis tellus id interdum velit. Aliquet porttitor lacus luctus accumsan. Proin libero nunc consequat interdum varius sit. A pellentesque sit amet porttitor eget dolor morbi non arcu. Nec sagittis aliquam malesuada bibendum arcu vitae elementum curabitur vitae. Mi proin sed libero enim sed faucibus turpis in. Tincidunt ornare massa eget egestas purus viverra accumsan in nisl. Tellus molestie nunc non blandit.

Fames ac turpis egestas maecenas pharetra convallis posuere. Eget egestas purus viverra accumsan in. In tellus integer feugiat scelerisque varius morbi enim. Pretium fusce id velit ut. Ante metus dictum at tempor commodo ullamcorper a lacus. Ut ornare lectus sit amet est placerat. Vitae purus faucibus ornare suspendisse sed. Nibh tortor id aliquet lectus. Nunc scelerisque viverra mauris in aliquam sem. Sed libero enim sed faucibus turpis in eu mi. Ut pharetra sit amet aliquam id diam. Diam maecenas ultricies mi eget mauris pharetra et ultrices neque. Ac felis donec et odio pellentesque diam volutpat commodo sed. Ut diam quam nulla porttitor massa. Duis tristique sollicitudin nibh sit amet commodo. Senectus et netus et malesuada fames ac turpis. Facilisi morbi tempus iaculis urna id volutpat lacus laoreet non. Euismod in pellentesque massa placerat duis ultricies lacus sed. Nulla facilisi etiam dignissim diam quis enim.

Euismod elementum nisi quis eleifend quam. Et malesuada fames ac turpis egestas. Pulvinar neque laoreet suspendisse interdum consectetur libero. Mollis nunc sed id semper risus. Enim praesent elementum facilisis leo vel fringilla. Leo urna molestie at elementum eu facilisis sed. Id aliquet lectus proin nibh nisl condimentum id venenatis. Amet consectetur adipiscing elit ut aliquam purus. Diam vulputate ut pharetra sit amet aliquam id diam. Scelerisque in dictum non consectetur a erat name. Euismod elementum nisi quis eleifend quam adipiscing vitae proin sagittis. Ultricies integer quis auctor elit sed. Elit eget gravida cum sociis natoque penatibus. Sed risus ultricies tristique nulla aliquet enim tortor at auctor. Egestas maecenas pharetra convallis posuere morbi leo urna molestie.
`

type LoopbackFSTestSuite struct {
	suite.Suite
	lfs *LoopbackFS
}

func (suite *LoopbackFSTestSuite) SetupTest() {
	lfs := NewLoopbackFSComponent()
	suite.lfs = lfs.(*LoopbackFS)
	suite.lfs.path = testPath

	err := log.SetDefaultLogger("silent", common.LogConfig{})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}

	err = os.MkdirAll(testPath, os.FileMode(0777))
	panicIfNotNil(err, "Failed to setup test directories")
	err = os.MkdirAll(filepath.Join(testPath, dirOne), os.FileMode(0777))
	panicIfNotNil(err, "Failed to setup test directories")
	err = os.MkdirAll(filepath.Join(testPath, dirEmpty), os.FileMode(0777))
	panicIfNotNil(err, "Failed to setup test directories")

	f, err := os.OpenFile(filepath.Join(testPath, fileLorem), os.O_RDWR|os.O_CREATE, os.FileMode(0777))
	panicIfNotNil(err, "Failed to setup test files")
	_, err = f.WriteString(loremText)
	panicIfNotNil(err, "Failed to setup test files")
	err = f.Close()
	panicIfNotNil(err, "Failed to setup test files")

	f, err = os.OpenFile(filepath.Join(testPath, fileHello), os.O_RDWR|os.O_CREATE, os.FileMode(0777))
	panicIfNotNil(err, "Failed to setup test files")
	err = f.Close()
	panicIfNotNil(err, "Failed to setup test files")

	err = suite.lfs.Start(context.Background())
	panicIfNotNil(err, "Failed to Start LoopbackFS component")
}

func (suite *LoopbackFSTestSuite) cleanupTest() {
	err := os.RemoveAll(testPath)
	panicIfNotNil(err, "Failed to tear down test directories")
}
func (suite *LoopbackFSTestSuite) TestCreateDir() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	err := suite.lfs.CreateDir(internal.CreateDirOptions{Name: dirTwo, Mode: os.FileMode(0755)})
	assert.NoError(err, "CreateDir: Failed")
	suite.DirExists(filepath.Join(testPath, dirTwo))
}

func (suite *LoopbackFSTestSuite) TestDeleteDir() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	err := suite.lfs.DeleteDir(internal.DeleteDirOptions{Name: dirEmpty})
	assert.NoError(err, "DeleteDir: Failed")
	suite.NoDirExists(filepath.Join(testPath, dirEmpty), "DeleteDir: Failed to delete")
}

func (suite *LoopbackFSTestSuite) TestStreamDir() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	info, _ := os.Stat(filepath.Join(testPath, fileLorem))

	attrs, _, err := suite.lfs.StreamDir(internal.StreamDirOptions{Name: dirOne})
	assert.NoError(err, "StreamDir: Failed")

	attr := attrs[0]

	assert.Equal("lorem.txt", attr.Name, "StreamDir: FileName not equal")
	assert.Equal(attr.Size, info.Size(), "StreamDir: File size not equal")
	assert.Equal(attr.Mode, info.Mode(), "StreamDir: File Mode not equal")
}

func (suite *LoopbackFSTestSuite) TestRenameDir() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	err := suite.lfs.RenameDir(internal.RenameDirOptions{Src: dirEmpty, Dst: "newempty"})
	assert.NoError(err, "RenameDir: Failed")

	suite.DirExists(filepath.Join(testPath, "newempty"))
}

func (suite *LoopbackFSTestSuite) TestCreateFile() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	handle, err := suite.lfs.CreateFile(internal.CreateFileOptions{Name: fileEmpty, Mode: os.FileMode(0644)})
	assert.NoError(err, "CreateFile: Failed")
	assert.NotNil(handle)

	assert.FileExists(filepath.Join(testPath, fileEmpty))

	err = suite.lfs.CloseFile(internal.CloseFileOptions{Handle: handle})
	assert.NoError(err, "CreateFile: Failed to close file")
}

func (suite *LoopbackFSTestSuite) TestDeleteFile() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	err := suite.lfs.DeleteFile(internal.DeleteFileOptions{Name: fileHello})
	assert.NoError(err, "DeleteFile: Failed")
	assert.NoFileExists(filepath.Join(testPath, fileHello), "DeleteFile: file was not deleted")
}

func (suite *LoopbackFSTestSuite) TestRenameFile() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	handle, err := suite.lfs.CreateFile(internal.CreateFileOptions{Name: fileEmpty, Mode: os.FileMode(0644)})
	assert.NoError(err, "TestRenameFile: Failed")
	assert.NotNil(handle)
	assert.FileExists(filepath.Join(testPath, fileEmpty))

	err = suite.lfs.CloseFile(internal.CloseFileOptions{Handle: handle})
	assert.NoError(err, "TestRenameFile: Failed to close file")

	err = suite.lfs.RenameFile(internal.RenameFileOptions{Src: fileEmpty, Dst: fileHello})
	assert.NoError(err)

	assert.FileExists(filepath.Join(testPath, fileHello))
	assert.NoFileExists(filepath.Join(testPath, fileEmpty))
}

func (suite *LoopbackFSTestSuite) TestRenameOpenFile() {
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping test on Windows")
		return
	}
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	handle, err := suite.lfs.CreateFile(internal.CreateFileOptions{Name: fileEmpty, Mode: os.FileMode(0644)})
	assert.NoError(err, "TestRenameOpenFile: Failed")
	assert.NotNil(handle)
	assert.FileExists(filepath.Join(testPath, fileEmpty))

	err = suite.lfs.CloseFile(internal.CloseFileOptions{Handle: handle})
	assert.NoError(err, "TestRenameOpenFile: Failed to close file")

	handle, err = suite.lfs.OpenFile(internal.OpenFileOptions{Name: fileEmpty, Flags: os.O_RDONLY, Mode: os.FileMode(0644)})
	assert.NoError(err, "TestRenameOpenFile: Failed to open file")
	assert.NotNil(handle)

	err = suite.lfs.RenameFile(internal.RenameFileOptions{Src: fileEmpty, Dst: fileHello})
	assert.NoError(err)

	assert.FileExists(filepath.Join(testPath, fileHello))
	assert.NoFileExists(filepath.Join(testPath, fileEmpty))

	err = suite.lfs.CloseFile(internal.CloseFileOptions{Handle: handle})
	assert.NoError(err, "TestRenameOpenFile: Failed to close file")
}

func (suite *LoopbackFSTestSuite) TestRenameWriteFile() {
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping test on Windows")
		return
	}
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	handle, err := suite.lfs.CreateFile(internal.CreateFileOptions{Name: fileEmpty, Mode: os.FileMode(0644)})
	assert.NoError(err, "TestRenameWriteFile: Failed")
	assert.NotNil(handle)
	assert.FileExists(filepath.Join(testPath, fileEmpty))

	err = suite.lfs.RenameFile(internal.RenameFileOptions{Src: fileEmpty, Dst: fileHello})
	assert.NoError(err)

	assert.FileExists(filepath.Join(testPath, fileHello))
	assert.NoFileExists(filepath.Join(testPath, fileEmpty))

	n, err := suite.lfs.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: []byte(quotesText)[:5]})
	assert.NoError(err)
	assert.Equal(5, n, "TestRenameWriteFile: failed to write the specified number of bytes")

	err = suite.lfs.CloseFile(internal.CloseFileOptions{Handle: handle})
	assert.NoError(err, "TestRenameWriteFile: Failed to close file")

	info, err := os.Stat(filepath.Join(testPath, fileHello))
	assert.NoError(err, "TestRenameWriteFile: cannot stat file")
	assert.Equal(int64(5), info.Size())
}

func (suite *LoopbackFSTestSuite) TestRenameWriteFileGetAttr() {
	if runtime.GOOS == "windows" {
		fmt.Println("Skipping test on Windows")
		return
	}
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	handle, err := suite.lfs.CreateFile(internal.CreateFileOptions{Name: fileEmpty, Mode: os.FileMode(0644)})
	assert.NoError(err, "TestRenameWriteFile: Failed")
	assert.NotNil(handle)
	assert.FileExists(filepath.Join(testPath, fileEmpty))

	n, err := suite.lfs.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: []byte(quotesText)[:5]})
	assert.NoError(err)
	assert.Equal(5, n, "TestRenameWriteFile: failed to write the specified number of bytes")

	attr, err := suite.lfs.GetAttr(internal.GetAttrOptions{Name: fileEmpty})
	assert.NoError(err)
	assert.EqualValues(5, attr.Size)

	err = suite.lfs.RenameFile(internal.RenameFileOptions{Src: fileEmpty, Dst: fileHello})
	assert.NoError(err)

	assert.FileExists(filepath.Join(testPath, fileHello))
	assert.NoFileExists(filepath.Join(testPath, fileEmpty))

	attr, err = suite.lfs.GetAttr(internal.GetAttrOptions{Name: fileHello})
	assert.NoError(err)
	assert.EqualValues(5, attr.Size)

	err = suite.lfs.CloseFile(internal.CloseFileOptions{Handle: handle})
	assert.NoError(err, "TestRenameWriteFile: Failed to close file")

	info, err := os.Stat(filepath.Join(testPath, fileHello))
	assert.NoError(err, "TestRenameWriteFile: cannot stat file")
	assert.Equal(int64(5), info.Size())
}

func (suite *LoopbackFSTestSuite) TestReadInBuffer() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	handle, err := suite.lfs.OpenFile(internal.OpenFileOptions{Name: fileLorem, Flags: os.O_RDONLY, Mode: os.FileMode(0644)})
	assert.NoError(err, "ReadInBuffer: Failed to open file")
	assert.NotNil(handle)
	testCases := []struct {
		offset int64
		data   []byte
		truth  []byte
	}{
		{
			offset: 0,
			data:   make([]byte, 20),
			truth:  []byte(loremText)[0:20],
		},
		{
			offset: 21,
			data:   make([]byte, 8),
			truth:  []byte(loremText)[21 : 21+8],
		},
	}

	for _, testCase := range testCases {
		n, err := suite.lfs.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle, Offset: testCase.offset, Data: testCase.data})
		assert.NoError(err)
		assert.Len(testCase.truth, n, "ReadInBuffer: number of bytes returned not equal to input size")
		assert.Equal(testCase.data, testCase.truth)
	}

	err = suite.lfs.CloseFile(internal.CloseFileOptions{Handle: handle})
	assert.NoError(err, "ReadInBuffer: Failed to close file")
}

func (suite *LoopbackFSTestSuite) TestWriteFile() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	handle, err := suite.lfs.OpenFile(internal.OpenFileOptions{Name: fileQuotes, Flags: os.O_RDWR | os.O_CREATE, Mode: os.FileMode(0644)})
	assert.NoError(err, "WriteFile: failed to open file")
	assert.NotNil(handle)

	n, err := suite.lfs.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: []byte(quotesText)[:5]})
	assert.NoError(err)
	assert.Equal(5, n, "WriteFile: failed to write the specified number of bytes")

	attr, err := suite.lfs.GetAttr(internal.GetAttrOptions{Name: fileQuotes})
	assert.NoError(err)
	assert.EqualValues(5, attr.Size)

	n, err = suite.lfs.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 5, Data: []byte(quotesText)[5:]})
	assert.NoError(err)
	assert.Len([]byte(quotesText)[5:], n, "WriteFile: failed to write specified number of bytes")

	err = suite.lfs.CloseFile(internal.CloseFileOptions{Handle: handle})
	assert.NoError(err, "WriteFile: Failed to close file")

	attr, err = suite.lfs.GetAttr(internal.GetAttrOptions{Name: fileQuotes})
	assert.NoError(err)
	assert.EqualValues(len(quotesText), attr.Size)
}

func (suite *LoopbackFSTestSuite) TestTruncateFile() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	handle, err := suite.lfs.OpenFile(internal.OpenFileOptions{Name: fileLorem, Flags: os.O_RDWR, Mode: os.FileMode(0644)})
	assert.NoError(err, "TruncateFile: failed to open file")
	assert.NotNil(handle)

	err = suite.lfs.TruncateFile(internal.TruncateFileOptions{Name: fileLorem, Size: 10})
	assert.NoError(err)
	info, err := os.Stat(filepath.Join(testPath, fileLorem))
	assert.NoError(err, "TruncateFile: cannot stat file")
	assert.Equal(int64(10), info.Size())

	err = suite.lfs.CloseFile(internal.CloseFileOptions{Handle: handle})
	assert.NoError(err, "TruncateFile: Failed to close file")
}

func (suite *LoopbackFSTestSuite) TestTruncateClosedFile() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	handle, err := suite.lfs.OpenFile(internal.OpenFileOptions{Name: fileLorem, Flags: os.O_RDWR, Mode: os.FileMode(0644)})
	assert.NoError(err, "TruncateFile: failed to open file")
	assert.NotNil(handle)

	err = suite.lfs.CloseFile(internal.CloseFileOptions{Handle: handle})
	assert.NoError(err, "TruncateFile: Failed to close file")

	err = suite.lfs.TruncateFile(internal.TruncateFileOptions{Name: fileLorem, Size: 10})
	assert.NoError(err)
	info, err := os.Stat(filepath.Join(testPath, fileLorem))
	assert.NoError(err, "TruncateFile: cannot stat file")
	assert.Equal(int64(10), info.Size())
}

func (suite *LoopbackFSTestSuite) TestGetAttr() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	attr, err := suite.lfs.GetAttr(internal.GetAttrOptions{Name: fileLorem})
	assert.NoError(err)
	info, err := os.Stat(filepath.Join(testPath, fileLorem))
	assert.NoError(err)

	assert.Equal(attr.Size, info.Size())
	assert.Equal(attr.Name, info.Name())
	assert.Equal(attr.Mode, info.Mode())
	assert.Equal(attr.IsDir(), info.IsDir())
}

func (suite *LoopbackFSTestSuite) TestStageAndCommitData() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	lfs := &LoopbackFS{}

	lfs.path = common.ExpandPath("~/blocklfstest")
	err := os.MkdirAll(lfs.path, os.FileMode(0755))
	assert.NoError(err)
	defer os.RemoveAll(lfs.path)

	err = lfs.StageData(internal.StageDataOptions{Name: "testBlock", Data: []byte(loremText), Id: "123"})
	assert.NoError(err)

	err = lfs.StageData(internal.StageDataOptions{Name: "testBlock", Data: []byte(loremText), Id: "456"})
	assert.NoError(err)

	err = lfs.StageData(internal.StageDataOptions{Name: "testBlock", Data: []byte(loremText), Id: "789"})
	assert.NoError(err)

	blockList := []string{"123", "789", "456"}
	err = lfs.CommitData(internal.CommitDataOptions{Name: "testBlock", List: blockList})
	assert.NoError(err)
}

func TestLoopbackFSTestSuite(t *testing.T) {
	suite.Run(t, new(LoopbackFSTestSuite))
}

func panicIfNotNil(err error, msg string) {
	if err != nil {
		panic(fmt.Sprintf("%s: err[%s]", err, msg))
	}
}
