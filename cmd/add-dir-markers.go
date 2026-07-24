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
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/component/s3storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
)

type addDirMarkersOptions struct {
	dryRun bool
}

type markerScanResult struct {
	objectsScanned int
	markersAdded   int
}

var addDirMarkersOpts addDirMarkersOptions

var addDirMarkersCmd = &cobra.Command{
	Use:     "add-dir-markers",
	Short:   "Add missing directory markers to an S3 bucket.",
	Long:    "Scans the configured S3 bucket or subdirectory and creates zero-byte marker objects for every implied directory that does not already have one.",
	GroupID: groupUtil,
	Args:    cobra.NoArgs,
	Example: `  # Preview missing markers
  cloudfuse add-dir-markers --config-file=config.yaml --dry-run

  # Add the missing markers
  cloudfuse add-dir-markers --config-file=config.yaml`,
	RunE: runAddDirMarkers,
}

func runAddDirMarkers(_ *cobra.Command, _ []string) error {
	if options.ConfigFile == "" {
		if _, err := os.Stat(common.DefaultConfigFilePath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("config file not provided")
			}
			return fmt.Errorf("failed to inspect default config file: %w", err)
		}
		options.ConfigFile = common.DefaultConfigFilePath
	}
	if err := parseConfig(); err != nil {
		return err
	}

	s3Options := s3storage.Options{}
	if err := config.UnmarshalKey("s3storage", &s3Options); err != nil {
		return fmt.Errorf("failed to read s3storage config: %w", err)
	}
	if strings.TrimSpace(s3Options.BucketName) == "" {
		return fmt.Errorf("s3storage.bucket-name must be set in the config")
	}

	_ = log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	comp := s3storage.News3storageComponent()
	if err := comp.Configure(true); err != nil {
		return fmt.Errorf("s3storage configure failed: %w", err)
	}
	if err := comp.Start(context.Background()); err != nil {
		return fmt.Errorf("s3storage start failed: %w", err)
	}
	defer func() { _ = comp.Stop() }()

	s3Component, ok := comp.(*s3storage.S3Storage)
	if !ok {
		return fmt.Errorf("unexpected s3storage component type")
	}
	client, ok := s3Component.Storage.(*s3storage.Client)
	if !ok {
		return fmt.Errorf("unexpected S3 client type")
	}

	var onMissingMarker func(string)
	if addDirMarkersOpts.dryRun {
		onMissingMarker = func(marker string) {
			fmt.Fprintln(os.Stdout, marker)
		}
	}

	result, err := backfillDirectoryMarkers(
		context.Background(),
		client.AwsS3Client,
		s3Options.BucketName,
		s3Options.PrefixPath,
		addDirMarkersOpts.dryRun,
		onMissingMarker,
	)
	if err != nil {
		return err
	}

	action := "added"
	if addDirMarkersOpts.dryRun {
		action = "would add"
	}
	location := strings.Trim(s3Options.PrefixPath, "/")
	if location == "" {
		location = "<bucket root>"
	}
	fmt.Fprintf(
		os.Stdout,
		"Directory marker scan complete for %s/%s: scanned %d objects, %s %d markers.\n",
		s3Options.BucketName,
		location,
		result.objectsScanned,
		action,
		result.markersAdded,
	)
	return nil
}

type directoryMarkerS3API interface {
	ListObjectsV2(
		context.Context,
		*s3.ListObjectsV2Input,
		...func(*s3.Options),
	) (*s3.ListObjectsV2Output, error)
	PutObject(
		context.Context,
		*s3.PutObjectInput,
		...func(*s3.Options),
	) (*s3.PutObjectOutput, error)
}

func backfillDirectoryMarkers(
	ctx context.Context,
	api directoryMarkerS3API,
	bucket string,
	prefix string,
	dryRun bool,
	onMissingMarker func(string),
) (markerScanResult, error) {
	result := markerScanResult{}
	scanPrefix := strings.Trim(prefix, "/")
	if scanPrefix != "" {
		scanPrefix += "/"
	}

	activeMarkers := make(map[string]struct{})
	var continuationToken *string
	for {
		output, err := api.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(scanPrefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return result, fmt.Errorf("failed to list objects under %q: %w", scanPrefix, err)
		}
		for _, object := range output.Contents {
			if object.Key == nil {
				continue
			}
			result.objectsScanned++
			key := aws.ToString(object.Key)
			if scanPrefix != "" && !strings.HasPrefix(key, scanPrefix) {
				continue
			}

			for marker := range activeMarkers {
				if !strings.HasPrefix(key, marker) {
					delete(activeMarkers, marker)
				}
			}
			if strings.HasSuffix(key, "/") {
				activeMarkers[key] = struct{}{}
			}

			for i := 0; i < len(key); i++ {
				if key[i] != '/' {
					continue
				}
				marker := key[:i+1]
				if len(marker) < len(scanPrefix) {
					continue
				}
				if _, found := activeMarkers[marker]; found {
					continue
				}

				activeMarkers[marker] = struct{}{}
				if onMissingMarker != nil {
					onMissingMarker(marker)
				}
				if !dryRun {
					_, err := api.PutObject(ctx, &s3.PutObjectInput{
						Bucket:        aws.String(bucket),
						Key:           aws.String(marker),
						Body:          bytes.NewReader(nil),
						ContentLength: aws.Int64(0),
					})
					if err != nil {
						return result, fmt.Errorf(
							"failed to create directory marker %q: %w",
							marker,
							err,
						)
					}
				}
				result.markersAdded++
			}
		}
		if !aws.ToBool(output.IsTruncated) {
			break
		}
		if output.NextContinuationToken == nil || *output.NextContinuationToken == "" {
			return result, fmt.Errorf(
				"S3 returned a truncated listing without a continuation token",
			)
		}
		continuationToken = output.NextContinuationToken
	}

	return result, nil
}

func init() {
	rootCmd.AddCommand(addDirMarkersCmd)
	addDirMarkersCmd.Flags().StringVarP(
		&options.ConfigFile,
		"config-file",
		"c",
		"",
		"Path to cloudfuse config file (default: ./config.yaml)",
	)
	addDirMarkersCmd.Flags().StringVarP(
		&options.PassPhrase,
		"passphrase",
		"p",
		"",
		"Base64 encoded key used to decrypt an encrypted config file.",
	)
	addDirMarkersCmd.Flags().BoolVar(
		&addDirMarkersOpts.dryRun,
		"dry-run",
		false,
		"List missing markers without creating them",
	)
}
