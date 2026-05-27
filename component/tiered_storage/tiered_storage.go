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

package tiered_storage

import (
	"context"
	"fmt"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/handlemap"
)

/* NOTES:
   - Component shall have a structure which inherits "internal.BaseComponent" to participate in pipeline
   - Component shall register a name and its constructor to participate in pipeline  (add by default by generator)
   - Order of calls : Constructor -> Configure -> Start ..... -> Stop
   - To read any new setting from config file follow the Configure method default comments
*/

// Common structure for Component
type TieredStorage struct {
	internal.BaseComponent
}

// Structure defining your config parameters
type TieredStorageOptions struct {
	// e.g. var1 uint32 `config:"var1"`
}

const compName = "tiered_storage"

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &TieredStorage{}

func (c *TieredStorage) Name() string {
	return compName
}

func (c *TieredStorage) SetName(name string) {
	c.BaseComponent.SetName(name)
}

func (c *TieredStorage) SetNextComponent(nc internal.Component) {
	c.BaseComponent.SetNextComponent(nc)
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (c *TieredStorage) Start(ctx context.Context) error {
	log.Trace("TieredStorage::Start : Starting component %s", c.Name())

	// TieredStorage : start code goes here

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (c *TieredStorage) Stop() error {
	log.Trace("TieredStorage::Stop : Stopping component %s", c.Name())

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (c *TieredStorage) Configure(_ bool) error {
	log.Trace("TieredStorage::Configure : %s", c.Name())

	// >> If you do not need any config parameters remove below code and return nil
	conf := TieredStorageOptions{}
	err := config.UnmarshalKey(c.Name(), &conf)
	if err != nil {
		log.Err("TieredStorage::Configure : config error [invalid config attributes]")
		return fmt.Errorf("TieredStorage: config error [invalid config attributes]")
	}
	// Extract values from 'conf' and store them as you wish here

	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (c *TieredStorage) OnConfigChange() {
}

// Directory operations
func (c *TieredStorage) CreateDir(options internal.CreateDirOptions) error {
	return nil
}

func (c *TieredStorage) DeleteDir(options internal.DeleteDirOptions) error {
	return nil
}

func (c *TieredStorage) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	return false
}

func (c *TieredStorage) OpenDir(options internal.OpenDirOptions) error {
	return nil
}

func (c *TieredStorage) StreamDir(
	options internal.StreamDirOptions,
) ([]*internal.ObjAttr, string, error) {
	return nil, "", nil
}

func (c *TieredStorage) CloseDir(options internal.CloseDirOptions) error {
	return nil
}

func (c *TieredStorage) RenameDir(options internal.RenameDirOptions) error {
	return nil
}

// File operations
func (c *TieredStorage) CreateFile(
	options internal.CreateFileOptions,
) (*handlemap.Handle, error) {
	return nil, nil
}

func (c *TieredStorage) DeleteFile(options internal.DeleteFileOptions) error {
	return nil
}

func (c *TieredStorage) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	return nil, nil
}

func (c *TieredStorage) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	return 0, nil
}

func (c *TieredStorage) WriteFile(options *internal.WriteFileOptions) (int, error) {
	return 0, nil
}

func (c *TieredStorage) SyncFile(options internal.SyncFileOptions) error {
	return nil
}

func (c *TieredStorage) FlushFile(options internal.FlushFileOptions) error {
	return nil
}

func (c *TieredStorage) ReleaseFile(options internal.ReleaseFileOptions) error {
	return nil
}

func (c *TieredStorage) RenameFile(options internal.RenameFileOptions) error {
	return nil
}

func (c *TieredStorage) SyncDir(options internal.SyncDirOptions) error {
	return nil
}

// Symlink operations
func (c *TieredStorage) CreateLink(options internal.CreateLinkOptions) error {
	return nil
}

func (c *TieredStorage) ReadLink(options internal.ReadLinkOptions) (string, error) {
	return "", nil
}

// Filesystem level operations
func (c *TieredStorage) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	return &internal.ObjAttr{}, nil
}

func (c *TieredStorage) Chmod(options internal.ChmodOptions) error {
	return nil
}

func (c *TieredStorage) Chown(options internal.ChownOptions) error {
	return nil
}

func (c *TieredStorage) TruncateFile(options internal.TruncateFileOptions) error {
	return nil
}

func (c *TieredStorage) FileUsed(name string) error {
	return nil
}

func (c *TieredStorage) StatFs() (*common.Statfs_t, bool, error) {
	return nil, false, nil
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewTieredStorageComponent() internal.Component {
	comp := &TieredStorage{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewTieredStorageComponent)
}
