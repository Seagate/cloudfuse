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

package file_cache

import (
	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
)

/*
This function is responsible for going through the fileOps map and servicing each file
*/
func (fc *FileCache) async_cloud_handler() {

	//use some sort of expoential backoff. not sure if there are better ways to do this but initially this seems ok
	//var timeout time.Duration = 10

	//check if cloud is up
	// if up, then go to sync map and service the file ops. reset the timeout
	// if not, then call sleep() and increase the timeout

	fc.fileOps.Range(func(key, value interface{}) bool {

		return true
	})

}

// File is already flushed locally, we just need to upload it to the cloud
func (fc *FileCache) asyncFlushFile(options internal.FlushFileOptions) error {

	localPath := common.JoinUnixFilepath(fc.tmpPath, options.Handle.Path)
	uploadHandle, err := common.Open(localPath)
	if err != nil {
		log.Err("FileCache::FlushFile : error [unable to open upload handle] %s [%s]", options.Handle.Path, err.Error())
		return nil
	}

	err = fc.NextComponent().CopyFromFile(
		internal.CopyFromFileOptions{
			Name: options.Handle.Path,
			File: uploadHandle,
		})

	uploadHandle.Close()
	if err != nil {
		log.Err("FileCache::FlushFile : %s upload failed [%s]", options.Handle.Path, err.Error())
		return err
	}

	return nil
}

func (fc *FileCache) asyncDeleteFile(options internal.DeleteFileOptions) error {

	err := fc.NextComponent().DeleteFile(options)
	err = fc.validateStorageError(options.Name, err, "DeleteFile", false)
	if err != nil {
		log.Err("FileCache::DeleteFile : error  %s [%s]", options.Name, err.Error())
		return err
	}
	return nil
}

func (fc *FileCache) asyncRenameFile(options internal.RenameFileOptions) error {

	err := fc.NextComponent().RenameFile(options)
	err = fc.validateStorageError(options.Src, err, "RenameFile", false)
	if err != nil {
		log.Err("FileCache::RenameFile : %s failed to rename file [%s]", options.Src, err.Error())
		return err
	}
	return nil
}

func (fc *FileCache) asyncDeleteDir(options internal.DeleteDirOptions) error {
	err := fc.NextComponent().DeleteDir(options)
	if err != nil {
		log.Err("FileCache::DeleteDir : %s failed", options.Name)
		// There is a chance that meta file for directory was not created in which case
		// rest api delete will fail while we still need to cleanup the local cache for the same
		return err
	}

	return nil
}

func (fc *FileCache) asyncRenameDir(options internal.RenameDirOptions) error {

	err := fc.NextComponent().RenameDir(options)
	if err != nil {
		log.Err("FileCache::RenameDir : error %s [%s]", options.Src, err.Error())
		return err
	}
	return nil

}
