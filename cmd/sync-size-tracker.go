/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates

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
	"fmt"
	"os"
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/component/s3storage"
	"github.com/Seagate/cloudfuse/component/size_tracker"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:    "sync-size-tracker",
	Hidden: true,
	Short:  "Update the size tracker journal with the size of the configured S3 subdirectory",
	Long:   "Reads s3storage.subdirectory from the provided config file, calculates the total size of all objects under it, and updates the size tracker journal.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if options.ConfigFile == "" {
			_, err := os.Stat(common.DefaultConfigFilePath)
			if err != nil && os.IsNotExist(err) {
				return fmt.Errorf("config file not provided")
			}
			options.ConfigFile = common.DefaultConfigFilePath
		}
		if err := parseConfig(); err != nil {
			return err
		}

		// Read subdirectory from config
		var dir string
		err := config.UnmarshalKey("s3storage.subdirectory", &dir)
		if err != nil {
			return fmt.Errorf("failed to read s3storage.subdirectory from config: %w", err)
		}
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return fmt.Errorf("s3storage.subdirectory must be set in the config")
		}
		dir = strings.TrimPrefix(dir, "/")

		// Build and start s3storage component
		comp := s3storage.News3storageComponent()
		if err := comp.Configure(true); err != nil {
			return fmt.Errorf("s3storage configure failed: %w", err)
		}
		ctx := context.Background()
		if err := comp.Start(ctx); err != nil {
			return fmt.Errorf("s3storage start failed: %w", err)
		}
		defer func() { _ = comp.Stop() }()

		s3c, ok := comp.(*s3storage.S3Storage)
		if !ok {
			return fmt.Errorf("unexpected s3storage component type")
		}

		total, err := sumS3PrefixRecursive(s3c, dir)
		if err != nil {
			return err
		}

		// Determine journal file name (use config override if present)
		journalName := "mount_size.dat"
		if config.IsSet("size_tracker.journal-name") {
			var jn string
			_ = config.UnmarshalKey("size_tracker.journal-name", &jn)
			jn = strings.TrimSpace(jn)
			if jn != "" {
				journalName = jn
			}
		}

		// Update journal
		ms, err := size_tracker.CreateSizeJournal(journalName)
		if err != nil {
			return fmt.Errorf("failed to open size journal: %w", err)
		}
		defer func() { _ = ms.CloseFile() }()

		current := ms.GetSize()
		if total > current {
			ms.Add(total - current)
		} else if total < current {
			ms.Subtract(current - total)
		}

		// Print minimal status
		var bucket string
		_ = config.UnmarshalKey("s3storage.bucket-name", &bucket)
		fmt.Printf("sync complete: bucket=%s prefix=%q size=%d (was %d) journal=%s\n",
			bucket, dir, total, current, journalName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
	// Removed --directory; we read the subdirectory from config instead.
	syncCmd.PersistentFlags().
		StringVar(&options.ConfigFile, "config-file", "", "Path to cloudfuse config file (default: ./config.yaml)")
}

// sumS3PrefixRecursive walks a prefix recursively and sums file sizes.
func sumS3PrefixRecursive(s3c *s3storage.S3Storage, prefix string) (uint64, error) {
	var total uint64
	prefix = internal.ExtendDirName(strings.TrimPrefix(prefix, "/"))
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(
				s3c.Storage.(*s3storage.Client).Config.AuthConfig.BucketName,
			),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		}
		entries, err := s3c.Storage.(*s3storage.Client).AwsS3Client.ListObjectsV2(
			context.Background(),
			input,
		)
		if err != nil {
			return 0, fmt.Errorf("listing objects failed: %w", err)
		}
		for _, e := range entries.Contents {
			// skip directory markers and any unexpected nil sizes
			if strings.HasSuffix(*e.Key, "/") || e.Size == nil {
				continue
			}
			total += uint64(*e.Size)
		}
		if entries.NextContinuationToken == nil {
			break
		}
		continuationToken = entries.NextContinuationToken
	}
	return total, nil
}
