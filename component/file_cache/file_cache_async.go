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
	"fmt"
	"syscall"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
)

/*
This function is responsible for going through the fileOps map and servicing each file
*/
func (fc *FileCache) async_cloud_handler() {

	//we can use the s3/azure sdk to retry cloud ops without having to implement our own timeout

	//check if cloud is up
	// if up, then go to sync map and service the file ops. reset the timeout
	// if not, then call sleep() and increase the timeout

	//not sure if this actually works, may have to look at other methods to iterate through sync map
	var returnVal error //race condition on returnVal?

	for {

		fc.fileOps.Range(func(key, value interface{}) bool {

			val, _ := fc.fileOps.Load(key)
			fmt.Printf("the value is %s for key %s\n", val, key)
			attributes := val.(FileAttributes)

			switch {
			case attributes.operation == "DeleteDir":
				returnVal = fc.asyncDeleteDir(attributes.options.(internal.DeleteDirOptions))

			case attributes.operation == "RenameDir":
				returnVal = fc.asyncRenameDir(attributes.options.(internal.RenameDirOptions))

			case attributes.operation == "CreateFile":
				returnVal = fc.asyncCreateFile(attributes.options.(internal.CreateFileOptions))

			case attributes.operation == "DeleteFile":
				returnVal = fc.asyncDeleteFile(attributes.options.(internal.DeleteFileOptions))

			case attributes.operation == "FlushFile":
				returnVal = fc.asyncFlushFile(attributes.options.(FlushFileAbstraction))

			case attributes.operation == "RenameFile":
				returnVal = fc.asyncRenameFile(attributes.options.(internal.RenameFileOptions))

			case attributes.operation == "Chmod":
				returnVal = fc.asyncChmod(attributes.options.(internal.ChmodOptions))

			case attributes.operation == "Chown":
				returnVal = fc.asyncChown(attributes.options.(internal.ChownOptions))

			case attributes.operation == "SyncFile":
				returnVal = fc.asyncSyncFile(attributes.options.(internal.SyncFileOptions))
			}

			if returnVal == nil {
				fmt.Print("returnVal/error not nil\n")
				_ = fc.fileOps.CompareAndDelete(key, attributes) //file has been serviced, remove it from map only if file op hasn't been updated
			}

			return true
		})
	}

}

// File is already flushed locally, we just need to upload it to the cloud
func (fc *FileCache) asyncFlushFile(options FlushFileAbstraction) error {

	localPath := common.JoinUnixFilepath(fc.tmpPath, options.Name)
	//fmt.Printf("path in async thread is %s\n", localPath)
	uploadHandle, err := common.Open(localPath)

	if err != nil {
		log.Err("FileCache::FlushFile : error [unable to open upload handle] %s [%s]", options.Name, err.Error())
		return err
	}

	err = fc.NextComponent().CopyFromFile(
		internal.CopyFromFileOptions{
			Name: options.Name,
			File: uploadHandle,
		})

	uploadHandle.Close()
	if err != nil {
		log.Err("FileCache::FlushFile : %s upload failed [%s]", options.Name, err.Error())
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

func (fc *FileCache) asyncCreateFile(options internal.CreateFileOptions) error {

	newF, err := fc.NextComponent().CreateFile(options)
	if err != nil {
		log.Err("FileCache::CreateFile : Failed to create file %s", options.Name)
		return err
	}
	newF.GetFileObject().Close()
	return nil
}

func (fc *FileCache) asyncChmod(options internal.ChmodOptions) error {

	//need to first flushFile before Chmod to ensure file is in cloud
	//could probably do this as a one liner but figure it out later if needed

	// flushFilePath := FlushFileAbstraction{}

	// flushFilePath.Name = options.Name

	// fc.asyncFlushFile(flushFilePath)

	err := fc.NextComponent().Chmod(options)
	err = fc.validateStorageError(options.Name, err, "Chmod", false)
	if err != nil {
		if err != syscall.EIO {
			log.Err("FileCache::Chmod : %s failed to change mode [%s]", options.Name, err.Error())
		} else {
			fc.missedChmodList.LoadOrStore(options.Name, true)
		}
		return err
	}
	return nil
}

func (fc *FileCache) asyncChown(options internal.ChownOptions) error {

	//need to first flushFile before Chmod to ensure file is in cloud
	//could probably do this as a one liner but figure it out later if needed
	flushFilePath := FlushFileAbstraction{}

	flushFilePath.Name = options.Name

	fc.asyncFlushFile(flushFilePath)

	err := fc.NextComponent().Chown(options)
	err = fc.validateStorageError(options.Name, err, "Chown", false)
	if err != nil {
		log.Err("FileCache::Chown : %s failed to change owner [%s]", options.Name, err.Error())
		return err
	}
	return nil
}

func (fc *FileCache) asyncSyncFile(options internal.SyncFileOptions) error {

	err := fc.NextComponent().SyncFile(options)
	if err != nil {
		log.Err("FileCache::SyncFile : %s failed", options.Handle.Path)
		return err
	}
	return nil
}
