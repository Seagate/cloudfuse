/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2026 Seagate Technology LLC and/or its Affiliates

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

package cmd

import (
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDirectoryMarkerS3 struct {
	pages   []*s3.ListObjectsV2Output
	listPos int
	putKeys []string
}

func (f *fakeDirectoryMarkerS3) ListObjectsV2(
	_ context.Context,
	_ *s3.ListObjectsV2Input,
	_ ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	page := f.pages[f.listPos]
	f.listPos++
	return page, nil
}

func (f *fakeDirectoryMarkerS3) PutObject(
	_ context.Context,
	input *s3.PutObjectInput,
	_ ...func(*s3.Options),
) (*s3.PutObjectOutput, error) {
	f.putKeys = append(f.putKeys, aws.ToString(input.Key))
	_, _ = io.ReadAll(input.Body)
	return &s3.PutObjectOutput{}, nil
}

func TestMissingDirectoryMarkers(t *testing.T) {
	keys := []string{
		"photos/2025/",
		"photos/2025/a.jpg",
		"photos/2025/events/b.jpg",
		"photos/empty/",
		"unrelated/path/file.txt",
	}

	markers := missingDirectoryMarkers(keys, "photos/")

	assert.Equal(t, []string{"photos/", "photos/2025/events/"}, markers)
}

func TestBackfillDirectoryMarkersDryRunHandlesPagination(t *testing.T) {
	api := &fakeDirectoryMarkerS3{pages: []*s3.ListObjectsV2Output{
		{
			Contents:              []types.Object{{Key: aws.String("a/b/file.txt")}},
			IsTruncated:           aws.Bool(true),
			NextContinuationToken: aws.String("page-2"),
		},
		{
			Contents:    []types.Object{{Key: aws.String("a/")}},
			IsTruncated: aws.Bool(false),
		},
	}}

	result, err := backfillDirectoryMarkers(context.Background(), api, "bucket", "", true)

	require.NoError(t, err)
	assert.Equal(t, 2, result.objectsScanned)
	assert.Equal(t, 1, result.markersAdded)
	assert.Equal(t, []string{"a/b/"}, result.missingMarkers)
	assert.Empty(t, api.putKeys)
}

func TestBackfillDirectoryMarkersCreatesMissingMarkers(t *testing.T) {
	api := &fakeDirectoryMarkerS3{pages: []*s3.ListObjectsV2Output{{
		Contents: []types.Object{{Key: aws.String("one/two/file.txt")}},
	}}}

	result, err := backfillDirectoryMarkers(context.Background(), api, "bucket", "", false)

	require.NoError(t, err)
	assert.Equal(t, 2, result.markersAdded)
	assert.Equal(t, []string{"one/", "one/two/"}, api.putKeys)
}
