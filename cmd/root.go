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

package cmd

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/spf13/cobra"
)

type GithubApiReleaseData struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

type Blob struct {
	XMLName xml.Name `xml:"Blob"`
	Name    string   `xml:"Name"`
}

var disableVersionCheck bool

var rootCmd = &cobra.Command{
	Use:               "cloudfuse",
	Short:             "Cloudfuse is an open source project developed to provide a virtual filesystem backed by cloud storage.",
	Long:              "Cloudfuse is an open source project developed to provide a virtual filesystem backed by cloud storage. It uses the FUSE protocol to communicate with the operating system, and implements filesystem operations using Azure or S3 cloud storage REST APIs.",
	Version:           common.CloudfuseVersion,
	FlagErrorHandling: cobra.ExitOnError,
	SilenceUsage:      true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !disableVersionCheck {
			err := VersionCheck()
			if err != nil {
				return err
			}
		}
		return errors.New("missing command options\n\nDid you mean this?\n\tcloudfuse mount\n\nRun 'cloudfuse --help' for usage")
	},
}

// getRemoteVersion : From public release get the latest cloudfuse version
func getRemoteVersion(req string) (string, error) {
	resp, err := http.Get(req)
	if err != nil {
		log.Err("getRemoteVersion: error getting release version from Github: [%s]", err.Error())
		return "", err
	}
	if resp.StatusCode != 200 {
		log.Err("getRemoteVersion: [got status %d from URL %s]", resp.StatusCode, req)
		return "", fmt.Errorf("unable to get latest version: GET %s failed with status %d", req, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Err("getRemoteVersion: error reading body of response [%s]", err.Error())
		return "", err
	}

	var releaseData GithubApiReleaseData
	err = json.Unmarshal(body, &releaseData)
	if err != nil {
		log.Err("getRemoteVersion: error parsing json response [%s]", err.Error())
		return "", err
	}

	// trim the leading "v"
	versionNumber := strings.TrimPrefix(releaseData.Name, "v")
	return versionNumber, nil
}

// beginDetectNewVersion : Get latest release version and compare if user needs an upgrade or not
func beginDetectNewVersion() chan interface{} {
	completed := make(chan interface{})
	stderr := os.Stderr
	go func() {
		defer close(completed)

		latestVersionUrl := common.CloudfuseReleaseURL + "/latest"
		remoteVersion, err := getRemoteVersion(latestVersionUrl)
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

		remote, err := common.ParseVersion(remoteVersion)
		if err != nil {
			log.Err("beginDetectNewVersion: error parsing remoteVersion [%s]", err.Error())
			completed <- err.Error()
			return
		}

		if local.OlderThan(*remote) {
			executablePathSegments := strings.Split(strings.ReplaceAll(os.Args[0], "\\", "/"), "/")
			executableName := executablePathSegments[len(executablePathSegments)-1]
			log.Info("beginDetectNewVersion: A new version of Cloudfuse is available. Current Version=%s, Latest Version=%s", common.CloudfuseVersion, remoteVersion)
			fmt.Fprintf(stderr, "*** "+executableName+": A new version [%s] is available. Consider upgrading to latest version for bug-fixes & new features. ***\n", remoteVersion)
			log.Info("*** "+executableName+": A new version [%s] is available. Consider upgrading to latest version for bug-fixes & new features. ***\n", remoteVersion)
			completed <- "A new version of Cloudfuse is available"
		}
	}()
	return completed
}

// VersionCheck : Start version check and wait for 8 seconds to complete otherwise just timeout and move on
func VersionCheck() error {
	select {
	//either wait till this routine completes or timeout if it exceeds 8 secs
	case <-beginDetectNewVersion():
	case <-time.After(8 * time.Second):
		return fmt.Errorf("unable to obtain latest version information. please check your internet connection")
	}
	return nil
}

// ignoreCommand : There are command implicitly added by cobra itself, while parsing we need to ignore these commands
func ignoreCommand(cmdArgs []string) bool {
	ignoreCmds := []string{"completion", "help"}
	if len(cmdArgs) > 0 {
		for _, c := range ignoreCmds {
			if c == cmdArgs[0] {
				return true
			}
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
	args := make([]string, 0)
	for i := 0; i < len(cmdArgs); i++ {
		// /etc/fstab will give everything in comma separated list with -o option
		if cmdArgs[i] == "-o" {
			i++
			if i < len(cmdArgs) {
				bfuseArgs := make([]string, 0)
				lfuseArgs := make([]string, 0)

				// Check if ',' exists in arguments or not. If so we assume it might be coming from /etc/fstab
				opts := strings.Split(cmdArgs[i], ",")
				for _, o := range opts {
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
	rootCmd.PersistentFlags().BoolVar(&disableVersionCheck, "disable-version-check", false, "To disable version check that is performed automatically")
}
