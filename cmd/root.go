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

package cmd

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/spf13/cobra"
)

type GithubApiReleaseData struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []asset `json:"assets"`
}

type releaseInfo struct {
	Version   string
	AssetURL  string
	AssetName string
	HashURL   string
}

type Blob struct {
	XMLName xml.Name `xml:"Blob"`
	Name    string   `xml:"Name"`
}

var disableVersionCheck bool

// Command group IDs for organizing help output (Cobra v1.6.0+)
const (
	groupCore   = "core"
	groupConfig = "config"
	groupUtil   = "util"
)

var rootCmd = &cobra.Command{
	Use:          "cloudfuse",
	Short:        "Cloudfuse is an open source project developed to provide a virtual filesystem backed by cloud storage.",
	Long:         "Cloudfuse is an open source project developed to provide a virtual filesystem backed by cloud storage. It uses the FUSE protocol to communicate with the operating system, and implements filesystem operations using Azure or S3 cloud storage REST APIs.",
	Version:      common.CloudfuseVersion,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !disableVersionCheck {
			err := VersionCheck()
			if err != nil {
				return err
			}
		}
		return errors.New(
			"missing command options\n\nDid you mean this?\n\tcloudfuse mount\n\nRun 'cloudfuse --help' for usage",
		)
	},
}

func getRelease(ctx context.Context, version string) (*releaseInfo, error) {
	url := common.CloudfuseReleaseURL + "/latest"
	if version != "" {
		url = fmt.Sprintf(common.CloudfuseReleaseURL+"/tags/v%s", version)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// explicitly request the response format we need (best practice)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Add Authorization header to raise rate limit
	githubApiToken := os.Getenv("GH_API_TOKEN")
	if githubApiToken != "" {
		req.Header.Set("Authorization", "token "+githubApiToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get release info: %s", resp.Status)
	}

	var rel GithubApiReleaseData
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}

	packageAsset, err := selectPackageAsset(rel.Assets, opt.Package)
	if err != nil {
		return nil, err
	}

	hashAsset, err := selectHashAsset(rel.Assets)
	// Only report an error if package is not exe since goreleaser does not provide a hash
	// for those releases.
	if err != nil && opt.Package != "exe" {
		return nil, err
	}

	return &releaseInfo{
		Version:   strings.TrimPrefix(rel.TagName, "v"),
		AssetURL:  packageAsset.BrowserDownloadURL,
		AssetName: packageAsset.Name,
		HashURL:   hashAsset.BrowserDownloadURL,
	}, nil
}

func selectPackageAsset(assets []asset, ext string) (*asset, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	if ext == "tar" {
		ext = "tar.gz"
	}

	for _, asset := range assets {
		if strings.HasPrefix(asset.Name, "cloudfuse") &&
			strings.Contains(asset.Name, osName) &&
			strings.Contains(asset.Name, arch) &&
			strings.HasSuffix(asset.Name, ext) {
			return &asset, nil
		}
	}

	return nil, errors.New("no suitable version of cloudfuse found for the current platform")
}

func selectHashAsset(assets []asset) (*asset, error) {
	for _, asset := range assets {
		if strings.Contains(asset.Name, "checksums_sha256") {
			return &asset, nil
		}
	}

	return nil, errors.New("no checksums found")
}

// beginDetectNewVersion : Get latest release version and compare if user needs an upgrade or not
func beginDetectNewVersion(ctx context.Context) chan any {
	completed := make(chan any)
	stderr := os.Stderr
	go func() {
		defer close(completed)

		latestRelease, err := getRelease(ctx, "")
		if err != nil {
			log.Err("beginDetectNewVersion: error getting latest version [%s]", err.Error())
			if strings.Contains(err.Error(), "no such host") {
				log.Err("beginDetectNewVersion: check your network connection and proxy settings")
			}
			completed <- err.Error()
			return
		}

		local, err := common.ParseVersion(common.CloudfuseVersion)
		if err != nil {
			log.Err("beginDetectNewVersion: error parsing CloudfuseVersion [%s]", err.Error())
			completed <- err.Error()
			return
		}

		remote, err := common.ParseVersion(latestRelease.Version)
		if err != nil {
			log.Err("beginDetectNewVersion: error parsing remoteVersion [%s]", err.Error())
			completed <- err.Error()
			return
		}

		if local.OlderThan(*remote) {
			executablePathSegments := strings.Split(strings.ReplaceAll(os.Args[0], "\\", "/"), "/")
			executableName := executablePathSegments[len(executablePathSegments)-1]
			log.Info(
				"beginDetectNewVersion: A new version of Cloudfuse is available. Current Version=%s, Latest Version=%s",
				common.CloudfuseVersion,
				latestRelease.Version,
			)
			fmt.Fprintf(
				stderr,
				"*** "+executableName+": A new version [%s] is available. Consider upgrading to latest version for bug-fixes & new features. ***\n",
				latestRelease.Version,
			)
			log.Info(
				"*** "+executableName+": A new version [%s] is available. Consider upgrading to latest version for bug-fixes & new features. ***\n",
				latestRelease.Version,
			)
			completed <- "A new version of Cloudfuse is available"
		}
	}()
	return completed
}

// VersionCheck : Start version check and wait for 8 seconds to complete otherwise just timeout and move on
func VersionCheck() error {
	// set an 8 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	completed := beginDetectNewVersion(ctx)
	select {
	//either wait till this routine completes or timeout if it exceeds 8 secs
	case <-completed:
		return nil
	case <-ctx.Done():
		return fmt.Errorf(
			"unable to obtain latest version information. please check your internet connection",
		)
	}
}

// ignoreCommand : There are command implicitly added by cobra itself, while parsing we need to ignore these commands
func ignoreCommand(cmdArgs []string) bool {
	ignoreCmds := []string{"completion", "help", "__complete", "__completeNoDesc"}
	if len(cmdArgs) > 0 {
		if slices.Contains(ignoreCmds, cmdArgs[0]) {
			return true
		}
	}
	return false
}

// parseArgs : Depending upon inputs are coming from /etc/fstab or CLI, parameter style may vary.
// -- /etc/fstab example : cloudfuse mount <dir> -o suid,nodev,--config-file=config.yaml,--use-adls=true,allow_other
// -- cli command        : cloudfuse mount <dir> -o suid,nodev --config-file=config.yaml --use-adls=true -o allow_other
// -- As we need to support both the ways, here we convert the /etc/fstab style (comma separated list) to standard cli ways
func parseArgs(cmdArgs []string) []string {
	// Ignore binary name, rest all are arguments to cloudfuse
	cmdArgs = cmdArgs[1:]

	cmd, _, err := rootCmd.Find(cmdArgs)
	if err != nil && cmd == rootCmd && !ignoreCommand(cmdArgs) {
		/* /etc/fstab has a standard format and it goes like "<binary> <mount point> <type> <options>"
		 * as here we can not give any subcommand like "cloudfuse mount" (giving this will assume mount is mount point)
		 * we need to assume 'mount' being default sub command.
		 * To do so, we just ignore the implicit commands handled by cobra and then try to check if input matches any of
		 * our subcommands or not. If not, we assume user has executed mount command without specifying mount subcommand
		 * so inject mount command in the cli options so that rest of the handling just assumes user gave mount subcommand.
		 */
		cmdArgs = append([]string{"mount"}, cmdArgs...)
	}

	// Check for /etc/fstab style inputs
	args := make([]string, 0, len(cmdArgs))
	for i := 0; i < len(cmdArgs); i++ {
		// /etc/fstab will give everything in comma separated list with -o option
		if cmdArgs[i] == "-o" {
			i++
			if i < len(cmdArgs) {
				var bfuseArgs, lfuseArgs []string

				// Check if ',' exists in arguments or not. If so we assume it might be coming from /etc/fstab
				opts := strings.SplitSeq(cmdArgs[i], ",")
				for o := range opts {
					// If we got comma separated list then all cloudfuse specific options needs to be extracted out
					//  as those shall not be part of -o list which for us means libfuse options
					if strings.HasPrefix(o, "--") {
						bfuseArgs = append(bfuseArgs, o)
					} else {
						lfuseArgs = append(lfuseArgs, o)
					}
				}

				// Extract and add libfuse options with -o
				if len(lfuseArgs) > 0 {
					args = append(args, "-o", strings.Join(lfuseArgs, ","))
				}

				// Extract and add cloudfuse specific options sepratly
				if len(bfuseArgs) > 0 {
					args = append(args, bfuseArgs...)
				}
			}
		} else {
			// If any option is without -o then keep it as is (assuming its directly from cli)
			args = append(args, cmdArgs[i])
		}
	}

	return args
}

// Execute : Actual command execution starts from here
func Execute() error {
	parsedArgs := parseArgs(os.Args)
	rootCmd.SetArgs(parsedArgs)

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
	return err
}

func init() {
	rootCmd.PersistentFlags().
		BoolVar(&disableVersionCheck, "disable-version-check", false, "To disable version check that is performed automatically")

	rootCmd.SetErrPrefix("cloudfuse error:")

	rootCmd.AddGroup(
		&cobra.Group{ID: groupCore, Title: "Core Commands:"},
		&cobra.Group{ID: groupConfig, Title: "Configuration Commands:"},
		&cobra.Group{ID: groupUtil, Title: "Utility Commands:"},
	)

	// Set the group for the built-in help and completion commands
	rootCmd.SetHelpCommandGroupID(groupUtil)
	rootCmd.SetCompletionCommandGroupID(groupUtil)
}
