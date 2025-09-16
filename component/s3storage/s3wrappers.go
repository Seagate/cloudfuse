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

package s3storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort" // to sort List() results
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/Seagate/cloudfuse/internal/convertname"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type getObjectOptions struct {
	name      string
	offset    int64
	count     int64
	isSymLink bool
	isDir     bool
}

type putObjectOptions struct {
	name       string
	objectData io.Reader
	size       int64
	isSymLink  bool
	isDir      bool
}

type copyObjectOptions struct {
	source    string
	target    string
	isSymLink bool
	isDir     bool
}

type renameObjectOptions struct {
	source    string
	target    string
	isSymLink bool
	isDir     bool
}

const symlinkStr = ".rclonelink"
const maxResultsPerListCall = 1000

// getObjectMultipartDownload downloads an object to a file using multipart download
// which can be much faster for large objects.
func (cl *Client) getObjectMultipartDownload(name string, fi *os.File) error {
	key := cl.getKey(name, false, false)
	log.Trace("Client::getObjectMultipartDownload : get object %s", key)

	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(cl.Config.AuthConfig.BucketName),
		Key:    aws.String(key),
	}

	if cl.Config.enableChecksum {
		getObjectInput.ChecksumMode = types.ChecksumModeEnabled
	}

	_, err := cl.downloader.Download(context.Background(), fi, getObjectInput)
	// check for errors
	if err != nil {
		attemptedAction := fmt.Sprintf("GetObject(%s)", key)
		return parseS3Err(err, attemptedAction)
	}
	return nil
}

// Wrapper for awsS3Client.GetObject.
// Set count = 0 to read to the end of the object.
// name is the path to the file.
func (cl *Client) getObject(options getObjectOptions) (io.ReadCloser, error) {
	key := cl.getKey(options.name, options.isSymLink, options.isDir)
	log.Trace("Client::getObject : get object %s (%d+%d)", key, options.offset, options.count)

	// deal with the range
	var rangeString string //string to be used to specify range of object to download from S3
	//TODO: add handle if the offset+count is greater than the end of Object.
	if options.count == 0 {
		// sending Range:"bytes=0-" gives errors from MinIO ("InvalidRange: The requested range is not satisfiable")
		// so if offset is 0 too, leave rangeString empty
		if options.offset != 0 {
			rangeString = "bytes=" + fmt.Sprint(options.offset) + "-"
		}
	} else {
		endRange := options.offset + options.count - 1
		rangeString = "bytes=" + fmt.Sprint(options.offset) + "-" + fmt.Sprint(endRange)
	}

	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(cl.Config.AuthConfig.BucketName),
		Key:    aws.String(key),
		Range:  aws.String(rangeString),
	}

	if cl.Config.enableChecksum {
		getObjectInput.ChecksumMode = types.ChecksumModeEnabled
	}

	result, err := cl.AwsS3Client.GetObject(context.Background(), getObjectInput)

	// check for errors
	if err != nil {
		attemptedAction := fmt.Sprintf("GetObject(%s)", key)
		return nil, parseS3Err(err, attemptedAction)
	}

	// return body, err
	return result.Body, err
}

// Wrapper for awsS3Client.PutObject.
// Pass in the name of the file, an io.Reader with the object data, the size of the upload,
// and whether the object is a symbolic link or not.
func (cl *Client) putObject(options putObjectOptions) error {
	key := cl.getKey(options.name, options.isSymLink, options.isDir)
	log.Trace("Client::putObject : putting object %s", key)
	ctx := context.Background()
	var err error

	putObjectInput := &s3.PutObjectInput{
		Bucket:      aws.String(cl.Config.AuthConfig.BucketName),
		Key:         aws.String(key),
		Body:        options.objectData,
		ContentType: aws.String(getContentType(key)),
	}

	if cl.Config.enableChecksum {
		putObjectInput.ChecksumAlgorithm = cl.Config.checksumAlgorithm
	}

	// If the object is small, just do a normal put object.
	// If not, then use a multipart upload
	if options.size < cl.Config.uploadCutoff {
		_, err = cl.AwsS3Client.PutObject(ctx, putObjectInput)
	} else {
		_, err = cl.uploader.Upload(ctx, putObjectInput)
	}

	attemptedAction := fmt.Sprintf("upload object %s", key)
	return parseS3Err(err, attemptedAction)
}

// Wrapper for awsS3Client.DeleteObject.
// name is the path to the file.
func (cl *Client) deleteObject(name string, isSymLink bool, isDir bool) error {
	key := cl.getKey(name, isSymLink, isDir)
	log.Trace("Client::deleteObject : deleting object %s", key)

	_, err := cl.AwsS3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(cl.Config.AuthConfig.BucketName),
		Key:    aws.String(key),
	})

	attemptedAction := fmt.Sprintf("delete object %s", key)
	return parseS3Err(err, attemptedAction)
}

// Wrapper for awsS3Client.DeleteObjects.
// names is a list of paths to the objects.
func (cl *Client) deleteObjects(objects []*internal.ObjAttr) error {
	if objects == nil {
		return nil
	}
	log.Trace("Client::deleteObjects : deleting %d objects", len(objects))
	// build list to send to DeleteObjects
	keyList := make([]types.ObjectIdentifier, len(objects))
	for i, object := range objects {
		key := cl.getKey(object.Path, object.IsSymlink(), object.IsDir())
		keyList[i] = types.ObjectIdentifier{
			Key: &key,
		}
	}
	// send keyList for deletion
	result, err := cl.AwsS3Client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{
		Bucket: &cl.Config.AuthConfig.BucketName,
		Delete: &types.Delete{
			Objects: keyList,
			Quiet:   aws.Bool(true),
		},
	})
	if err != nil {
		log.Err(
			"Client::DeleteDirectory : Failed to delete %d files. Here's why: %v",
			len(objects),
			err,
		)
		if result != nil {
			for i := 0; i < len(result.Errors); i++ {
				log.Err(
					"Client::DeleteDirectory : Failed to delete key %s. Here's why: %s",
					result.Errors[i].Key,
					result.Errors[i].Message,
				)
			}
		}
	}

	return err
}

// Wrapper for awsS3Client.HeadObject.
// HeadObject() acts just like GetObject, except no contents are returned.
// So this is used to get metadata / attributes for an object.
// name is the path to the file.
func (cl *Client) headObject(name string, isSymlink bool, isDir bool) (*internal.ObjAttr, error) {
	key := cl.getKey(name, isSymlink, isDir)
	log.Trace("Client::headObject : object %s", key)

	result, err := cl.AwsS3Client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(cl.Config.AuthConfig.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		// Make sure the attempted starts with "HeadObject",  or else parseS3Err will log to Err
		attemptedAction := fmt.Sprintf("HeadObject(%s)", name)
		return nil, parseS3Err(err, attemptedAction)
	}

	var object *internal.ObjAttr

	if isDir {
		object = createObjAttrDir(name)
	} else {
		object = createObjAttr(name, *result.ContentLength, *result.LastModified, isSymlink)
	}

	return object, nil
}

// Wrapper for awsS3Client.HeadBucket
func (cl *Client) headBucket(bucketName string) (*s3.HeadBucketOutput, error) {
	headBucketOutput, err := cl.AwsS3Client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	return headBucketOutput, parseS3Err(err, "HeadBucket "+bucketName)
}

// Wrapper for awsS3Client.CopyObject
func (cl *Client) copyObject(options copyObjectOptions) error {
	// copy the object to its new key
	sourceKey := cl.getKey(options.source, options.isSymLink, options.isDir)
	targetKey := cl.getKey(options.target, options.isSymLink, options.isDir)

	copyObjectInput := &s3.CopyObjectInput{
		Bucket: aws.String(cl.Config.AuthConfig.BucketName),
		CopySource: aws.String(
			fmt.Sprintf("%v/%v", cl.Config.AuthConfig.BucketName, url.PathEscape(sourceKey)),
		),
		Key: aws.String(targetKey),
	}

	if cl.Config.enableChecksum {
		copyObjectInput.ChecksumAlgorithm = cl.Config.checksumAlgorithm
	}

	_, err := cl.AwsS3Client.CopyObject(context.Background(), copyObjectInput)
	// check for errors on copy
	if err != nil {
		attemptedAction := fmt.Sprintf("copy %s to %s", sourceKey, targetKey)
		return parseS3Err(err, attemptedAction)
	}

	return err
}

func (cl *Client) renameObject(options renameObjectOptions) error {
	err := cl.copyObject(
		copyObjectOptions(options),
	) //nolint
	if err != nil {
		log.Err(
			"Client::renameObject : copyObject(%s->%s) failed. Here's why: %v",
			options.source,
			options.target,
			err,
		)
		return err
	}
	// Copy of the file is done so now delete the older file
	// in this case we don't need to check if the file exists, so we use deleteObject, not DeleteFile
	// this is what S3's DeleteObject spec is meant for: to make sure the object doesn't exist anymore
	err = cl.deleteObject(options.source, options.isSymLink, options.isDir)
	if err != nil {
		log.Err(
			"Client::renameObject : deleteObject(%s) failed. Here's why: %v",
			options.source,
			err,
		)
	}

	return err
}

// abortMultipartUpload stops a multipart upload and verifys that the parts are deleted.
func (cl *Client) abortMultipartUpload(key string, uploadID string) error {
	_, abortErr := cl.AwsS3Client.AbortMultipartUpload(
		context.Background(),
		&s3.AbortMultipartUploadInput{
			Bucket:   aws.String(cl.Config.AuthConfig.BucketName),
			Key:      aws.String(key),
			UploadId: &uploadID,
		},
	)
	if abortErr != nil {
		log.Err("Client::StageAndCommit : Error aborting multipart upload: ", abortErr.Error())
	}

	// AWS states you need to call listparts to verify that multipart upload was properly aborted
	resp, listErr := cl.AwsS3Client.ListParts(context.Background(), &s3.ListPartsInput{
		Bucket:   aws.String(cl.Config.AuthConfig.BucketName),
		Key:      aws.String(key),
		UploadId: &uploadID,
	})
	if len(resp.Parts) != 0 {
		log.Err(
			"Client::StageAndCommit : Error aborting multipart upload. There are parts remaining in the object with key: %s, uploadId: %s ",
			key,
			uploadID,
		)
	}
	if listErr != nil {
		log.Err(
			"Client::StageAndCommit : Error calling list parts. Unable to verify if multipart upload was properly aborted with key: %s, uploadId: %s, error: ",
			key,
			uploadID,
			abortErr.Error(),
		)
	}
	return errors.Join(abortErr, listErr)
}

// Wrapper for awsS3Client.ListBuckets
func (cl *Client) ListBuckets() ([]string, error) {
	log.Trace("Client::ListBuckets : Listing buckets")

	cntList := make([]string, 0)

	result, err := cl.AwsS3Client.ListBuckets(context.Background(), &s3.ListBucketsInput{})

	if err != nil {
		log.Err("Client::ListBuckets : Failed to list buckets. Here's why: %v", err)
		return cntList, err
	}

	for _, bucket := range result.Buckets {
		cntList = append(cntList, *bucket.Name)
	}

	return cntList, nil
}

// Wrapper for awsS3Client.ListObjectsV2
// List : Get a list of objects matching the given prefix, up to the next "/", similar to listing a directory.
// For predictable results, include the trailing slash in the prefix.
// When prefix has no trailing slash, List has unintuitive behavior (e.g. prefix "file" would match "filet-o-fish").
// This fetches the list using a marker so the caller code should handle marker logic.
// If count=0 - fetch max entries.
// the *string being returned is the token / marker and will be nil when the listing is complete.
func (cl *Client) List(
	prefix string,
	marker *string,
	count int32,
) ([]*internal.ObjAttr, *string, error) {
	log.Trace("Client::List : prefix %s, marker %s, count %d", prefix, func(marker *string) string {
		if marker != nil {
			return *marker
		}
		return ""
	}(marker), count)

	// prepare parameters
	bucketName := cl.Config.AuthConfig.BucketName
	if count == 0 {
		count = maxResultsPerListCall
	}

	// combine the configured prefix and the prefix being given to List to get a full listPath
	listPath := cl.getKey(prefix, false, false)
	// replace any trailing forward slash stripped by common.JoinUnixFilepath
	if (prefix != "" && prefix[len(prefix)-1] == '/') ||
		(prefix == "" && cl.Config.prefixPath != "") {
		listPath += "/"
	}

	// Only look for CommonPrefixes (subdirectories) if List was called with a prefix ending in a slash.
	// If prefix does not end in a slash, CommonPrefixes would find unwanted results.
	// For example, it would find "filet-of-fish/" when searching for "file".
	// Check for an empty path to prevent indexing to [-1]
	findCommonPrefixes := listPath == "" || listPath[len(listPath)-1] == '/'

	var nextMarker *string
	var token *string

	// using paginator from here: https://aws.github.io/aws-sdk-go-v2/docs/making-requests/#using-paginators
	// List is a tricky function. Here is a great explanation of how list works:
	// 	https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-prefixes.html

	if marker != nil && *marker == "" {
		token = nil
		// when called without a token, S3 returns the directory being listed as the first entry
		// but when we list a directory, we only want the directory's *contents*
		// so we need to ask for one more entry than we want
		count++
	} else {
		token = marker
	}
	params := &s3.ListObjectsV2Input{
		Bucket:            aws.String(bucketName),
		MaxKeys:           &count,
		Prefix:            aws.String(listPath),
		Delimiter:         aws.String("/"), // delimiter limits results and provides CommonPrefixes
		ContinuationToken: token,
	}
	paginator := s3.NewListObjectsV2Paginator(cl.AwsS3Client, params)
	// initialize list to be returned
	objectAttrList := make([]*internal.ObjAttr, 0)
	// fetch and process a single result page
	output, err := paginator.NextPage(context.Background())
	if err != nil {
		log.Err(
			"Client::List : Failed to list objects in bucket %v with prefix %v. Here's why: %v",
			prefix,
			bucketName,
			err,
		)
		return objectAttrList, nil, err
	}

	if output.IsTruncated != nil && *output.IsTruncated {
		nextMarker = output.NextContinuationToken
	} else {
		nextMarker = nil
	}

	// documentation for this S3 data structure:
	// 	https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3@v1.30.2#ListObjectsV2Output
	for _, value := range output.Contents {
		if *value.Key == listPath {
			continue
		}

		// push object info into the list
		name, isSymLink := cl.getFile(*value.Key)

		path := split(cl.Config.prefixPath, name)
		attr := createObjAttr(path, *value.Size, *value.LastModified, isSymLink)
		objectAttrList = append(objectAttrList, attr)
	}

	if findCommonPrefixes {
		// documentation for CommonPrefixes:
		// 	https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3@v1.30.2/types#CommonPrefix
		// create a map to keep track of all directories
		var dirList = make(map[string]bool)
		for _, value := range output.CommonPrefixes {
			dir := *value.Prefix
			dirList[dir] = true
			// let's extract and add intermediate directories
			// first cut the listPath (the full prefix path) off of the directory path
			_, intermediatePath, listPathFound := strings.Cut(dir, listPath)
			// if the listPath isn't here, that's weird
			if !listPathFound {
				log.Warn("Prefix mismatch with path %v when listing objects in %v.", dir, listPath)
			}
			// get an array of intermediate directories
			intermediateDirectories := strings.Split(intermediatePath, "/")
			// walk up the tree and add each one until we find an already existing parent
			// we have to iterate in descending order
			suffixToTrim := ""
			for i := len(intermediateDirectories) - 1; i >= 0; i-- {
				// ignore empty strings (split does not omit them)
				if intermediateDirectories[i] == "" {
					continue
				}
				// add to the suffix we're trimming off
				suffixToTrim = intermediateDirectories[i] + "/" + suffixToTrim
				// get the trimmed (parent) directory
				parentDir := strings.TrimSuffix(dir, suffixToTrim)
				// have we seen this one already?
				if dirList[parentDir] {
					break
				}
				dirList[parentDir] = true
			}
		}

		// now let's add attributes for all the directories in dirList
		for dir := range dirList {
			dirName, _ := cl.getFile(dir)
			if internal.TruncateDirName(dirName) == internal.TruncateDirName(listPath) {
				continue
			}
			path := split(cl.Config.prefixPath, dirName)
			attr := internal.CreateObjAttrDir(path)
			objectAttrList = append(objectAttrList, attr)
		}
	}

	// values should be returned in ascending order by key
	// sort the list before returning it
	sort.Slice(objectAttrList, func(i, j int) bool {
		return objectAttrList[i].Path < objectAttrList[j].Path
	})

	log.Debug("Client::List : %s returning %d entries", prefix, len(objectAttrList))

	return objectAttrList, nextMarker, nil
}

// create an object attributes struct
func createObjAttr(
	path string,
	size int64,
	lastModified time.Time,
	isSymLink bool,
) (attr *internal.ObjAttr) {
	attr = &internal.ObjAttr{
		Path:   path,
		Name:   filepath.Base(path),
		Size:   size,
		Mode:   0,
		Mtime:  lastModified,
		Atime:  lastModified,
		Ctime:  lastModified,
		Crtime: lastModified,
		Flags:  internal.NewFileBitMap(),
	}
	// set flags
	attr.Flags.Set(internal.PropFlagModeDefault)

	attr.Metadata = make(map[string]*string)

	if isSymLink {
		attr.Flags.Set(internal.PropFlagSymlink)
		attr.Metadata[symlinkKey] = to.Ptr("true")
	}

	return attr
}

// create an object attributes struct for a directory
func createObjAttrDir(path string) (attr *internal.ObjAttr) { //nolint
	// strip any trailing slash
	path = internal.TruncateDirName(path)
	// For these dirs we get only the name and no other properties so hardcoding time to current time
	currentTime := time.Now()

	attr = createObjAttr(path, 4096, currentTime, false)
	// Change the relevant fields for a directory
	attr.Mode = os.ModeDir
	// set flags
	attr.Flags = internal.NewDirBitMap()
	attr.Flags.Set(internal.PropFlagModeDefault)

	return attr
}

// getKey converts a file name to an object name. If it is a symlink it prepends
// .rclonelink. If it is set to convert names from Linux to Windows then it allows
// special characters like "*:<>?| to be displayed on Windows.
func (cl *Client) getKey(name string, isSymLink bool, isDir bool) string {
	if isSymLink {
		name = name + symlinkStr
	}

	name = common.JoinUnixFilepath(cl.Config.prefixPath, name)
	if runtime.GOOS == "windows" && cl.Config.restrictedCharsWin {
		name = convertname.WindowsFileToCloud(name)
	}

	// Directories in S3 end in a trailing slash
	if isDir {
		name = internal.ExtendDirName(name)
	}
	return name
}

// getFile converts an object name to a file name. If the name has a ".rclonelink" suffix.
// then it removes the suffix and returns true to indicate a symbolic link. If it is set to
// convert names from Linux to Windows then it converts special ASCII characters back to the
// original special characters.
func (cl *Client) getFile(name string) (string, bool) {
	isSymLink := false

	//todo: write a test the catches the out of bounds issue.
	if !cl.Config.disableSymlink && strings.HasSuffix(name, symlinkStr) {
		isSymLink = true
		name = name[:len(name)-len(symlinkStr)]
	}

	if runtime.GOOS == "windows" && cl.Config.restrictedCharsWin {
		name = convertname.WindowsCloudToFile(name)
	}

	return name, isSymLink
}
