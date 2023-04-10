package s3storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort" // to sort List() results
	"strings"
	"time"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Wrapper for awsS3Client.GetObject.
// Set count = 0 to read to the end of the object.
// key is the full path to the object (with the prefixPath).
func (cl *Client) getObject(name string, offset int64, count int64) (body io.ReadCloser, err error) {
	key := cl.getKey(name)
	log.Trace("Client::getObject : get object %s (%d+%d)", key, offset, count)

	// deal with the range
	var rangeString string //string to be used to specify range of object to download from S3
	//TODO: add handle if the offset+count is greater than the end of Object.
	if count == 0 {
		// sending Range:"bytes=0-" gives errors from Lyve Cloud ("InvalidRange: The requested range is not satisfiable")
		// so if offset is 0 too, leave rangeString empty
		if offset != 0 {
			rangeString = "bytes=" + fmt.Sprint(offset) + "-"
		}
	} else {
		endRange := offset + count
		rangeString = "bytes=" + fmt.Sprint(offset) + "-" + fmt.Sprint(endRange)
	}

	result, err := cl.awsS3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(cl.Config.authConfig.BucketName),
		Key:    aws.String(key),
		Range:  aws.String(rangeString),
	})

	// check for errors
	if err != nil {
		attemptedAction := fmt.Sprintf("GetObject(%s)", key)
		return nil, parseS3Err(err, attemptedAction)
	}

	// return body
	return result.Body, err
}

// Wrapper for awsS3Client.PutObject.
// Takes an io.Reader to work with both files and byte arrays.
// key is the full path to the object (with the prefixPath).
func (cl *Client) putObject(name string, objectData io.Reader) (err error) {
	key := cl.getKey(name)
	log.Trace("Client::putObject : putting object %s", key)

	// TODO: decide when to use this higher-level API
	// uploader := manager.NewUploader(cl.Client, func(u *manager.Uploader) {
	//  // TODO: Move this variable into the config file
	// 	u.PartSize = partMiBs * 1024 * 1024
	// })

	_, err = cl.awsS3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(cl.Config.authConfig.BucketName),
		Key:         aws.String(key),
		Body:        objectData,
		ContentType: aws.String(getContentType(key)),
	})

	attemptedAction := fmt.Sprintf("upload object %s", key)
	return parseS3Err(err, attemptedAction)
}

// Wrapper for awsS3Client.DeleteObject.
// key is the full path to the object (with the prefixPath).
func (cl *Client) deleteObject(name string) (err error) {
	key := cl.getKey(name)
	log.Trace("Client::deleteObject : deleting object %s", key)

	_, err = cl.awsS3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(cl.Config.authConfig.BucketName),
		Key:    aws.String(key),
	})

	attemptedAction := fmt.Sprintf("delete object %s", key)
	return parseS3Err(err, attemptedAction)
}

// Wrapper for awsS3Client.DeleteObjects.
// names is a list of paths to the objects.
func (cl *Client) deleteObjects(names []string) (err error) {
	log.Trace("Client::deleteObjects : deleting %d objects", len(names))
	// build list to send to DeleteObjects
	keyList := make([]types.ObjectIdentifier, len(names))
	for i, name := range names {
		key := cl.getKey(name)
		keyList[i] = types.ObjectIdentifier{
			Key: &key,
		}
	}
	// send keyList for deletion
	result, err := cl.awsS3Client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: &cl.Config.authConfig.BucketName,
		Delete: &types.Delete{
			Objects: keyList,
			Quiet:   true,
		},
	})
	if err != nil {
		log.Err("Client::DeleteDirectory : Failed to delete %d files. Here's why: %v", len(names), err)
		for i := 0; i < len(result.Errors); i++ {
			log.Err("Client::DeleteDirectory : Failed to delete key %s. Here's why: %s", result.Errors[i].Key, result.Errors[i].Message)
		}
	}

	return
}

// Wrapper for awsS3Client.HeadObject.
// HeadObject() acts just like GetObject, except no contents are returned.
// So this is used to get metadata / attributes for an object.
// name is the path to the file
func (cl *Client) headObject(name string) (attr *internal.ObjAttr, err error) {
	key := cl.getKey(name)
	log.Trace("Client::headObject : object %s", key)

	result, err := cl.awsS3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(cl.Config.authConfig.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		attemptedAction := fmt.Sprintf("HeadObject(%s)", name)
		return nil, parseS3Err(err, attemptedAction)
	}

	return createObjAttr(name, result.ContentLength, *result.LastModified), nil
}

// Wrapper for awsS3Client.CopyObject
func (cl *Client) copyObject(source string, target string) error {
	// copy the object to its new key
	sourceKey := cl.getKey(source)
	targetKey := cl.getKey(target)
	_, err := cl.awsS3Client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket: aws.String(cl.Config.authConfig.BucketName),
		// TODO: URL-encode CopySource
		CopySource: aws.String(fmt.Sprintf("%v/%v", cl.Config.authConfig.BucketName, sourceKey)),
		Key:        aws.String(targetKey),
	})
	// check for errors on copy
	if err != nil {
		attemptedAction := fmt.Sprintf("copy %s to %s", sourceKey, targetKey)
		return parseS3Err(err, attemptedAction)
	}

	return err
}

// Wrapper for awsS3Client.ListBuckets
func (cl *Client) ListBuckets() ([]string, error) {
	log.Trace("Client::ListBuckets : Listing buckets")

	cntList := make([]string, 0)

	result, err := cl.awsS3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})

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
func (cl *Client) List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error) {
	log.Trace("Client::List : prefix %s, marker %s", prefix, func(marker *string) string {
		if marker != nil {
			return *marker
		}
		return ""
	}(marker))

	// prepare parameters
	bucketName := cl.Config.authConfig.BucketName
	if count == 0 {
		count = common.MaxDirListCount
	}

	// combine the configured prefix and the prefix being given to List to get a full listPath
	listPath := cl.getKey(prefix)
	// replace any trailing forward slash stripped by common.JoinUnixFilepath
	if (prefix != "" && prefix[len(prefix)-1] == '/') || (prefix == "" && cl.Config.prefixPath != "") {
		listPath += "/"
	}

	// Only look for CommonPrefixes (subdirectories) if List was called with a prefix ending in a slash.
	// If prefix does not end in a slash, CommonPrefixes would find unwanted results.
	// For example, it would find "filet-of-fish/" when searching for "file".
	// Check for an empty path to prevent indexing to [-1]
	findCommonPrefixes := listPath == "" || listPath[len(listPath)-1] == '/'

	// create a map to keep track of all directories
	var dirList = make(map[string]bool)

	// using paginator from here: https://aws.github.io/aws-sdk-go-v2/docs/making-requests/#using-paginators
	// List is a tricky function. Here is a great explanation of how list works:
	// 	https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-prefixes.html
	params := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucketName),
		MaxKeys:   count,
		Prefix:    aws.String(listPath),
		Delimiter: aws.String("/"), // delimiter limits results and provides CommonPrefixes
	}
	paginator := s3.NewListObjectsV2Paginator(cl.awsS3Client, params)
	// initialize list to be returned
	objectAttrList := make([]*internal.ObjAttr, 0)
	// fetch and process result pages
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(context.TODO())
		if err != nil {
			log.Err("Client::List : Failed to list objects in bucket %v with prefix %v. Here's why: %v", prefix, bucketName, err)
			return objectAttrList, nil, err
		}
		// documentation for this S3 data structure:
		// 	https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3@v1.30.2#ListObjectsV2Output
		for _, value := range output.Contents {
			// push object info into the list
			path := split(cl.Config.prefixPath, *value.Key)
			attr := createObjAttr(path, value.Size, *value.LastModified)
			objectAttrList = append(objectAttrList, attr)
		}

		if findCommonPrefixes {
			// documentation for CommonPrefixes:
			// 	https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3@v1.30.2/types#CommonPrefix
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
		}
	}

	// now let's add attributes for all the directories in dirList
	for dir := range dirList {
		if dir == listPath {
			continue
		}
		path := split(cl.Config.prefixPath, dir)
		attr := createObjAttrDir(path)
		objectAttrList = append(objectAttrList, attr)
	}

	// values should be returned in ascending order by key
	// sort the list before returning it
	sort.Slice(objectAttrList, func(i, j int) bool {
		return objectAttrList[i].Path < objectAttrList[j].Path
	})

	newMarker := ""
	return objectAttrList, &newMarker, nil
}

// create an object attributes struct
func createObjAttr(path string, size int64, lastModified time.Time) (attr *internal.ObjAttr) {
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
	attr.Flags.Set(internal.PropFlagMetadataRetrieved)
	attr.Flags.Set(internal.PropFlagModeDefault)
	attr.Metadata = make(map[string]string)

	return attr
}

// create an object attributes struct for a directory
func createObjAttrDir(path string) (attr *internal.ObjAttr) {
	// strip any trailing slash
	path = internal.TruncateDirName(path)
	// For these dirs we get only the name and no other properties so hardcoding time to current time
	currentTime := time.Now()

	attr = createObjAttr(path, 4096, currentTime)
	// Change the relevant fields for a directory
	attr.Mode = os.ModeDir
	// set flags
	attr.Flags = internal.NewDirBitMap()
	attr.Flags.Set(internal.PropFlagMetadataRetrieved)
	attr.Flags.Set(internal.PropFlagModeDefault)

	return attr
}

// Convert file name to object getKey
func (cl *Client) getKey(name string) string {
	return common.JoinUnixFilepath(cl.Config.prefixPath, name)
}
