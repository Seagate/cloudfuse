/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
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

package cmd

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/spf13/cobra"
)

// Options for the CLI update command
type Options struct {
	Output  string // output path
	Version string
	Package string // package format: tar, deb, rpm
}

var opt = Options{}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type releaseInfo struct {
	Version   string
	AssetURL  string
	AssetName string
	HashURL   string
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the cloudfuse binary.",
	Long:  "Update the cloudfuse binary.",
	RunE: func(command *cobra.Command, args []string) error {
		if opt.Package == "" {
			packageFormat, err := determinePackageFormat()
			if err != nil {
				return fmt.Errorf("unable to determine package format: %w", err)
			}
			opt.Package = packageFormat
		}

		switch runtime.GOOS {
		case "linux":
			if opt.Package != "tar" && opt.Package != "deb" && opt.Package != "rpm" {
				return errors.New("--package should be one of tar|deb|rpm")
			}
			if os.Geteuid() != 0 && opt.Output == "" &&
				(opt.Package == "deb" || opt.Package == "rpm") {
				return errors.New(".deb and .rpm requires elevated privileges")
			}
			if opt.Output == "" && opt.Package == "tar" {
				return errors.New(
					"need to pass parameter --package with deb or rpm, or pass parameter --output with location to download to",
				)
			}

		case "windows":
			if opt.Package != "exe" && opt.Package != "zip" {
				return errors.New("--package should be one of exe|zip")
			}
			if opt.Output == "" && (opt.Package == "zip") {
				return errors.New(
					"need to pass parameter --package with exe or zip, or pass parameter --output with location to download to",
				)
			}

		default:
			return errors.New("unsupported OS, only Linux and Windows are supported")
		}

		if err := installUpdate(context.Background(), &opt); err != nil {
			return fmt.Errorf("error: %v", err)
		}
		return nil
	},
}

// installUpdate performs the self-update
func installUpdate(ctx context.Context, opt *Options) error {
	relInfo, err := getRelease(ctx, opt.Version, "")
	if err != nil {
		return fmt.Errorf("unable to detect new version: %w", err)
	}

	if relInfo.Version == common.CloudfuseVersion {
		fmt.Println("cloudfuse is up to date")
		return nil
	}

	fileName, err := downloadUpdate(ctx, relInfo, opt.Output)
	if err != nil {
		return fmt.Errorf("unable to download release: %w", err)
	}

	// Only verify hash for Linux releases as Windows releases are not hashed by goreleaser
	if opt.Package != "exe" {
		if err := verifyHash(ctx, fileName, relInfo.AssetName, relInfo.HashURL); err != nil {
			return fmt.Errorf("unable to verify checksum: %w", err)
		}
	}

	if opt.Output != "" {
		return nil
	}

	if runtime.GOOS == "windows" {
		return runWindowsInstaller(fileName)
	}

	return runLinuxInstaller(fileName)
}

func determinePackageFormat() (string, error) {
	if runtime.GOOS == "windows" {
		return "exe", nil
	}
	if hasCommand("apt") {
		return "deb", nil
	} else if hasCommand("rpm") {
		return "rpm", nil
	} else {
		return "", errors.New("neither apt nor rpm found, cannot determine package format")
	}
}

func hasCommand(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// runWindowsInstaller runs the Windows executable installer. Requires the user to restart the machine to apply changes.
func runWindowsInstaller(fileName string) error {
	absPath, err := filepath.Abs(fileName)
	if err != nil {
		return fmt.Errorf("unable to get absolute path: %w", err)
	}

	args := []string{
		"/SP-",
		"/VERYSILENT",
		"/NOICONS",
		"/NORESTART",
		"/SUPPRESSMSGBOXES",
		"/dir=expand:{autopf}\\Cloudfuse",
	}

	cmd := exec.Command(absPath, args...)

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run installer: %w", err)
	}

	fmt.Println(
		"Cloudfuse was successfully updated. Please restart the machine to apply the changes.",
	)

	return nil
}

// runLinuxInstaller installs the deb or rpm package
func runLinuxInstaller(fileName string) error {
	var packageCommand string
	if strings.HasSuffix(fileName, "deb") {
		packageCommand = "apt"
	} else if strings.HasSuffix(fileName, "rpm") {
		packageCommand = "rpm"
	} else {
		return errors.New("unsupported package format")
	}

	cmd := exec.Command(packageCommand, "-i", fileName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %s: %v", packageCommand, err)
	}
	return nil
}

// downloadUpdate downloads the update file
func downloadUpdate(ctx context.Context, relInfo *releaseInfo, output string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, relInfo.AssetURL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get release info: %s", resp.Status)
	}

	var installFile *os.File
	if output != "" {
		installFile, err = os.Create(output)
	} else {
		installFile, err = os.CreateTemp("", "cloudfuse-update-*"+relInfo.AssetName)
	}
	if err != nil {
		return "", fmt.Errorf("unable to create file: %w", err)
	}
	defer installFile.Close()

	if _, err = io.Copy(installFile, resp.Body); err != nil {
		return "", fmt.Errorf("unable to copy file: %w", err)
	}

	return installFile.Name(), nil
}

func getRelease(ctx context.Context, version string, testURL string) (*releaseInfo, error) {
	// use a dummy server when testing
	releaseUrl := common.CloudfuseReleaseURL
	if testURL != "" {
		releaseUrl = testURL
	}

	// use a specific version when provided
	url := releaseUrl + "/latest"
	if version != "" {
		url = fmt.Sprintf("%s/tags/v%s", releaseUrl, version)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
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

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}

	asset, err := selectPackageAsset(rel.Assets, opt.Package)
	if err != nil {
		return nil, err
	}

	hashAsset, err := downloadHashAsset(rel.Assets)
	// Only report an error if package is not exe since goreleaser does not provide a hash
	// for those releases.
	if err != nil && opt.Package != "exe" {
		return nil, err
	}

	return &releaseInfo{
		Version:   strings.TrimPrefix(rel.TagName, "v"),
		AssetURL:  asset.BrowserDownloadURL,
		AssetName: asset.Name,
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

func downloadHashAsset(assets []asset) (*asset, error) {
	for _, asset := range assets {
		if strings.Contains(asset.Name, "checksums_sha256") {
			return &asset, nil
		}
	}

	return nil, errors.New("no checksums found")
}

func findChecksum(packageName string, checksumTable string) (string, error) {
	lines := strings.Split(checksumTable, "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == packageName {
			return parts[0], nil
		}
	}
	return "", errors.New("checksum not found for the given file name")
}

func verifyHash(ctx context.Context, fileName, packageName, hashURL string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hashURL, nil)
	if err != nil {
		return fmt.Errorf("unable to download checksum file: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("unable to download checksum file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get checksum file: %s", resp.Status)
	}

	hashes := new(strings.Builder)
	if _, err = io.Copy(hashes, resp.Body); err != nil {
		return fmt.Errorf("failed to get checksum file: %s", resp.Status)
	}

	expectedHash, err := findChecksum(packageName, hashes.String())
	if err != nil {
		return fmt.Errorf("failed to get checksum file: %s", resp.Status)
	}

	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("unable to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("unable to compute hash: %w", err)
	}

	computedHash := hex.EncodeToString(hash.Sum(nil))
	if computedHash != expectedHash {
		return errors.New("checksum mismatch")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.PersistentFlags().
		StringVar(&opt.Output, "output", "", "Save the downloaded binary at a given path (default: replace running binary)")
	updateCmd.PersistentFlags().
		StringVar(&opt.Version, "version", "", "Install the given cloudfuse version (default: latest)")
	updateCmd.PersistentFlags().
		StringVar(&opt.Package, "package", "", "Package format: tar|deb|rpm|zip|exe (default: automatically detect package format)")
}
