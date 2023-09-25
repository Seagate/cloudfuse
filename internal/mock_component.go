/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

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

// Code generated by MockGen. DO NOT EDIT.
// Source: cloudfuse/internal (interfaces: Component)

// Package internal is a generated GoMock package.
package internal

import (
	context "context"
	reflect "reflect"

	common "github.com/Seagate/cloudfuse/common"
	handlemap "github.com/Seagate/cloudfuse/internal/handlemap"

	gomock "github.com/golang/mock/gomock"
)

var _ Component = &MockComponent{}

// MockComponent is a mock of Component interface.
type MockComponent struct {
	ctrl     *gomock.Controller
	recorder *MockComponentMockRecorder
}

// MockComponentMockRecorder is the mock recorder for MockComponent.
type MockComponentMockRecorder struct {
	mock *MockComponent
}

// NewMockComponent creates a new mock instance.
func NewMockComponent(ctrl *gomock.Controller) *MockComponent {
	mock := &MockComponent{ctrl: ctrl}
	mock.recorder = &MockComponentMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockComponent) EXPECT() *MockComponentMockRecorder {
	return m.recorder
}

// Chmod mocks base method.
func (m *MockComponent) Chmod(arg0 ChmodOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Chmod", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Chmod indicates an expected call of Chmod.
func (mr *MockComponentMockRecorder) Chmod(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Chmod", reflect.TypeOf((*MockComponent)(nil).Chmod), arg0)
}

// Chown mocks base method.
func (m *MockComponent) Chown(arg0 ChownOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Chown", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Chown indicates an expected call of Chown.
func (mr *MockComponentMockRecorder) Chown(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Chown", reflect.TypeOf((*MockComponent)(nil).Chown), arg0)
}

// CloseDir mocks base method.
func (m *MockComponent) CloseDir(arg0 CloseDirOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseDir", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseDir indicates an expected call of CloseDir.
func (mr *MockComponentMockRecorder) CloseDir(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseDir", reflect.TypeOf((*MockComponent)(nil).CloseDir), arg0)
}

// CloseFile mocks base method.
func (m *MockComponent) CloseFile(arg0 CloseFileOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseFile", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseFile indicates an expected call of CloseFile.
func (mr *MockComponentMockRecorder) CloseFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseFile", reflect.TypeOf((*MockComponent)(nil).CloseFile), arg0)
}

// Configure mocks base method.
func (m *MockComponent) Configure(arg0 bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Configure", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Configure indicates an expected call of Configure.
func (mr *MockComponentMockRecorder) Configure(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Configure", reflect.TypeOf((*MockComponent)(nil).Configure), arg0)
}

// CopyFromFile mocks base method.
func (m *MockComponent) CopyFromFile(arg0 CopyFromFileOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CopyFromFile", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// CopyFromFile indicates an expected call of CopyFromFile.
func (mr *MockComponentMockRecorder) CopyFromFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CopyFromFile", reflect.TypeOf((*MockComponent)(nil).CopyFromFile), arg0)
}

// CopyToFile mocks base method.
func (m *MockComponent) CopyToFile(arg0 CopyToFileOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CopyToFile", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// CopyToFile indicates an expected call of CopyToFile.
func (mr *MockComponentMockRecorder) CopyToFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CopyToFile", reflect.TypeOf((*MockComponent)(nil).CopyToFile), arg0)
}

// CreateDir mocks base method.
func (m *MockComponent) CreateDir(arg0 CreateDirOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDir", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateDir indicates an expected call of CreateDir.
func (mr *MockComponentMockRecorder) CreateDir(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDir", reflect.TypeOf((*MockComponent)(nil).CreateDir), arg0)
}

// CreateFile mocks base method.
func (m *MockComponent) CreateFile(arg0 CreateFileOptions) (*handlemap.Handle, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateFile", arg0)
	ret0, _ := ret[0].(*handlemap.Handle)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateFile indicates an expected call of CreateFile.
func (mr *MockComponentMockRecorder) CreateFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateFile", reflect.TypeOf((*MockComponent)(nil).CreateFile), arg0)
}

// CreateLink mocks base method.
func (m *MockComponent) CreateLink(arg0 CreateLinkOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateLink", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateLink indicates an expected call of CreateLink.
func (mr *MockComponentMockRecorder) CreateLink(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateLink", reflect.TypeOf((*MockComponent)(nil).CreateLink), arg0)
}

// DeleteDir mocks base method.
func (m *MockComponent) DeleteDir(arg0 DeleteDirOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteDir", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteDir indicates an expected call of DeleteDir.
func (mr *MockComponentMockRecorder) DeleteDir(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteDir", reflect.TypeOf((*MockComponent)(nil).DeleteDir), arg0)
}

// DeleteFile mocks base method.
func (m *MockComponent) DeleteFile(arg0 DeleteFileOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteFile", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteFile indicates an expected call of DeleteFile.
func (mr *MockComponentMockRecorder) DeleteFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteFile", reflect.TypeOf((*MockComponent)(nil).DeleteFile), arg0)
}

// SyncFile mocks base method.
func (m *MockComponent) SyncDir(arg0 SyncDirOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SyncDir", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// FlushFile indicates an expected call of FlushFile.
func (mr *MockComponentMockRecorder) SyncDir(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SyncDir", reflect.TypeOf((*MockComponent)(nil).SyncDir), arg0)
}

// SyncFile mocks base method.
func (m *MockComponent) SyncFile(arg0 SyncFileOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SyncFile", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// FlushFile indicates an expected call of FlushFile.
func (mr *MockComponentMockRecorder) SyncFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SyncFile", reflect.TypeOf((*MockComponent)(nil).SyncFile), arg0)
}

// FlushFile mocks base method.
func (m *MockComponent) FlushFile(arg0 FlushFileOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FlushFile", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// FlushFile indicates an expected call of FlushFile.
func (mr *MockComponentMockRecorder) FlushFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FlushFile", reflect.TypeOf((*MockComponent)(nil).FlushFile), arg0)
}

// GetAttr mocks base method.
func (m *MockComponent) GetAttr(arg0 GetAttrOptions) (*ObjAttr, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAttr", arg0)
	ret0, _ := ret[0].(*ObjAttr)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAttr indicates an expected call of GetAttr.
func (mr *MockComponentMockRecorder) GetAttr(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAttr", reflect.TypeOf((*MockComponent)(nil).GetAttr), arg0)
}

// InvalidateObject mocks base method.
func (m *MockComponent) InvalidateObject(arg0 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "InvalidateObject", arg0)
}

// InvalidateObject indicates an expected call of InvalidateObject.
func (mr *MockComponentMockRecorder) InvalidateObject(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InvalidateObject", reflect.TypeOf((*MockComponent)(nil).InvalidateObject), arg0)
}

// GetFileBlockOffsets mocks base method.
func (m *MockComponent) GetFileBlockOffsets(arg0 GetFileBlockOffsetsOptions) (*common.BlockOffsetList, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetFileBlockOffsets", arg0)
	ret0, _ := ret[0].(*common.BlockOffsetList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetFileBlockOffsets maps offsets to block ids.
func (mr *MockComponentMockRecorder) GetFileBlockOffsets(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetFileBlockOffsets", reflect.TypeOf((*MockComponent)(nil).GetFileBlockOffsets), arg0)
}

// IsDirEmpty mocks base method.
func (m *MockComponent) IsDirEmpty(arg0 IsDirEmptyOptions) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsDirEmpty", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsDirEmpty indicates an expected call of IsDirEmpty.
func (mr *MockComponentMockRecorder) IsDirEmpty(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsDirEmpty", reflect.TypeOf((*MockComponent)(nil).IsDirEmpty), arg0)
}

// Name mocks base method.
func (m *MockComponent) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockComponentMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockComponent)(nil).Name))
}

// NextComponent mocks base method.
func (m *MockComponent) NextComponent() Component {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NextComponent")
	ret0, _ := ret[0].(Component)
	return ret0
}

// NextComponent indicates an expected call of NextComponent.
func (mr *MockComponentMockRecorder) NextComponent() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NextComponent", reflect.TypeOf((*MockComponent)(nil).NextComponent))
}

// Get stats of cloudfuse mount.
func (m *MockComponent) StatFs() (*common.Statfs_t, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StatFs")
	ret0, _ := ret[0].(*common.Statfs_t)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Get stats of cloudfuse mount.
func (mr *MockComponentMockRecorder) StatFs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StatFs", reflect.TypeOf((*MockComponent)(nil).StatFs))
}

// OpenDir mocks base method.
func (m *MockComponent) OpenDir(arg0 OpenDirOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OpenDir", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// OpenDir indicates an expected call of OpenDir.
func (mr *MockComponentMockRecorder) OpenDir(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OpenDir", reflect.TypeOf((*MockComponent)(nil).OpenDir), arg0)
}

// OpenFile mocks base method.
func (m *MockComponent) OpenFile(arg0 OpenFileOptions) (*handlemap.Handle, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OpenFile", arg0)
	ret0, _ := ret[0].(*handlemap.Handle)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OpenFile indicates an expected call of OpenFile.
func (mr *MockComponentMockRecorder) OpenFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OpenFile", reflect.TypeOf((*MockComponent)(nil).OpenFile), arg0)
}

// Priority mocks base method.
func (m *MockComponent) Priority() ComponentPriority {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Priority")
	ret0, _ := ret[0].(ComponentPriority)
	return ret0
}

// Priority indicates an expected call of Priority.
func (mr *MockComponentMockRecorder) Priority() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Priority", reflect.TypeOf((*MockComponent)(nil).Priority))
}

// ReadDir mocks base method.
func (m *MockComponent) ReadDir(arg0 ReadDirOptions) ([]*ObjAttr, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadDir", arg0)
	ret0, _ := ret[0].([]*ObjAttr)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadDir mocks base method.
func (m *MockComponent) StreamDir(arg0 StreamDirOptions) ([]*ObjAttr, string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StreamDir", arg0)
	ret0, _ := ret[0].([]*ObjAttr)
	ret1, _ := ret[1].(error)
	return ret0, "", ret1
}

// ReadDir indicates an expected call of ReadDir.
func (mr *MockComponentMockRecorder) ReadDir(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadDir", reflect.TypeOf((*MockComponent)(nil).ReadDir), arg0)
}

// ReadFile mocks base method.
func (m *MockComponent) ReadFile(arg0 ReadFileOptions) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadFile", arg0)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadFile indicates an expected call of ReadFile.
func (mr *MockComponentMockRecorder) ReadFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadFile", reflect.TypeOf((*MockComponent)(nil).ReadFile), arg0)
}

// ReadInBuffer mocks base method.
func (m *MockComponent) ReadInBuffer(arg0 ReadInBufferOptions) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadInBuffer", arg0)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadInBuffer indicates an expected call of ReadInBuffer.
func (mr *MockComponentMockRecorder) ReadInBuffer(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadInBuffer", reflect.TypeOf((*MockComponent)(nil).ReadInBuffer), arg0)
}

// ReadLink mocks base method.
func (m *MockComponent) ReadLink(arg0 ReadLinkOptions) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadLink", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadLink indicates an expected call of ReadLink.
func (mr *MockComponentMockRecorder) ReadLink(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadLink", reflect.TypeOf((*MockComponent)(nil).ReadLink), arg0)
}

// ReleaseFile mocks base method.
func (m *MockComponent) ReleaseFile(arg0 ReleaseFileOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReleaseFile", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// ReleaseFile indicates an expected call of ReleaseFile.
func (mr *MockComponentMockRecorder) ReleaseFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReleaseFile", reflect.TypeOf((*MockComponent)(nil).ReleaseFile), arg0)
}

// RenameDir mocks base method.
func (m *MockComponent) RenameDir(arg0 RenameDirOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RenameDir", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RenameDir indicates an expected call of RenameDir.
func (mr *MockComponentMockRecorder) RenameDir(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RenameDir", reflect.TypeOf((*MockComponent)(nil).RenameDir), arg0)
}

// RenameFile mocks base method.
func (m *MockComponent) RenameFile(arg0 RenameFileOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RenameFile", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RenameFile indicates an expected call of RenameFile.
func (mr *MockComponentMockRecorder) RenameFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RenameFile", reflect.TypeOf((*MockComponent)(nil).RenameFile), arg0)
}

// SetAttr mocks base method.
func (m *MockComponent) SetAttr(arg0 SetAttrOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetAttr", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetAttr indicates an expected call of SetAttr.
func (mr *MockComponentMockRecorder) SetAttr(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetAttr", reflect.TypeOf((*MockComponent)(nil).SetAttr), arg0)
}

// SetName mocks base method.
func (m *MockComponent) SetName(arg0 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetName", arg0)
}

// SetName indicates an expected call of SetName.
func (mr *MockComponentMockRecorder) SetName(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetName", reflect.TypeOf((*MockComponent)(nil).SetName), arg0)
}

// SetNextComponent mocks base method.
func (m *MockComponent) SetNextComponent(arg0 Component) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetNextComponent", arg0)
}

// SetNextComponent indicates an expected call of SetNextComponent.
func (mr *MockComponentMockRecorder) SetNextComponent(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetNextComponent", reflect.TypeOf((*MockComponent)(nil).SetNextComponent), arg0)
}

// Start mocks base method.
func (m *MockComponent) Start(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockComponentMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockComponent)(nil).Start), arg0)
}

// Stop mocks base method.
func (m *MockComponent) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockComponentMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockComponent)(nil).Stop))
}

// TruncateFile mocks base method.
func (m *MockComponent) TruncateFile(arg0 TruncateFileOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TruncateFile", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// TruncateFile indicates an expected call of TruncateFile.
func (mr *MockComponentMockRecorder) TruncateFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TruncateFile", reflect.TypeOf((*MockComponent)(nil).TruncateFile), arg0)
}

// UnlinkFile mocks base method.
func (m *MockComponent) UnlinkFile(arg0 UnlinkFileOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnlinkFile", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// UnlinkFile indicates an expected call of UnlinkFile.
func (mr *MockComponentMockRecorder) UnlinkFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnlinkFile", reflect.TypeOf((*MockComponent)(nil).UnlinkFile), arg0)
}

// WriteFile mocks base method.
func (m *MockComponent) WriteFile(arg0 WriteFileOptions) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteFile", arg0)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WriteFile indicates an expected call of WriteFile.
func (mr *MockComponentMockRecorder) WriteFile(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteFile", reflect.TypeOf((*MockComponent)(nil).WriteFile), arg0)
}

// FileUsed mocks base method.
func (m *MockComponent) FileUsed(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FileUsed", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// FileUsed indicates an expected call to FileUsed.
func (mr *MockComponentMockRecorder) FileUsed(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FileUsed", reflect.TypeOf((*MockComponent)(nil).FileUsed), arg0)
}
