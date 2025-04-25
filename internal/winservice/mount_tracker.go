//go:build windows

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

package winservice

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Mount struct {
	MountPath  string `json:"mountPath"`
	ConfigFile string `json:"configFile"`
}

type Mounts struct {
	Mounts []Mount `json:"mounts"`
}

const mountFile = "mounts.json"

func getAppDataFolder(useSystem bool) (string, error) {
	if useSystem {
		systemRoot := os.Getenv("SystemRoot")
		if systemRoot == "" {
			return "", errors.New("Could not find system root")
		}
		systemRoot = filepath.Clean(systemRoot)
		fullPath := filepath.Join(systemRoot, "System32", "config", "systemprofile", "AppData", "Roaming", "Cloudfuse")
		return fullPath, nil
	}

	appDataPath, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(appDataPath, "Cloudfuse")
	return fullPath, nil
}

func getMountTrackerFile(useSystem bool) (string, error) {
	appDataPath, err := getAppDataFolder(useSystem)
	if err != nil {
		return "", err
	}

	// Check local file exists for this offset and file combination or not
	root, err := os.OpenRoot(appDataPath)
	if err != nil {
		return "", err
	}
	defer root.Close()

	// If the file does not exist, then create it
	_, err = root.Stat(mountFile)
	if err != nil && os.IsNotExist(err) {
		f, err := root.Create(mountFile)
		if err != nil {
			return "", err
		}

		data, err := json.MarshalIndent(Mounts{}, "", " ")
		if err != nil {
			return "", err
		}

		_, err = f.Write(data)
		if err != nil {
			return "", err
		}
	}
	fullPath := filepath.Join(appDataPath, mountFile)

	return fullPath, nil
}

func readMounts(useSystem bool) (Mounts, error) {
	trackerFile, err := getMountTrackerFile(useSystem)
	if err != nil {
		return Mounts{}, err
	}

	file, err := os.ReadFile(trackerFile)
	if err != nil {
		return Mounts{}, err
	}

	var mounts Mounts
	err = json.Unmarshal(file, &mounts)
	return mounts, err
}

func writeMounts(mounts Mounts, useSystem bool) error {
	trackerFile, err := getMountTrackerFile(useSystem)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(mounts, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(trackerFile, data, 0644)
}

func removeMount(mounts Mounts, mountPath string) Mounts {
	filtered := make([]Mount, 0)
	for _, mount := range mounts.Mounts {
		if mount.MountPath != mountPath {
			filtered = append(filtered, mount)
		}
	}

	filteredMounts := Mounts{Mounts: filtered}
	return filteredMounts
}

// DeleteMountJSONFiles deletes all the mount.json files on the system
func DeleteMountJSONFiles() error {
	trackerFile, err := getMountTrackerFile(false)
	if err != nil {
		return err
	}

	err = os.Remove(trackerFile)
	if err != nil {
		return err
	}

	trackerFile, err = getMountTrackerFile(true)
	if err != nil {
		return err
	}

	err = os.Remove(trackerFile)
	return err
}

// AddMountJSON adds an entry to our json file with the mount path and config
// file location.
func AddMountJSON(mountPath string, configFile string, useSystem bool) error {
	mounts, err := readMounts(useSystem)
	if err != nil {
		return err
	}

	// If a the path to the current mount is already in the mounts.json file
	// then remove it.
	mounts = removeMount(mounts, mountPath)

	newMount := Mount{MountPath: mountPath, ConfigFile: configFile}
	mounts.Mounts = append(mounts.Mounts, newMount)

	return writeMounts(mounts, useSystem)
}

// RemoveMountJSON removes an entry to from our json file.
func RemoveMountJSON(mountPath string, useSystem bool) error {
	mounts, err := readMounts(useSystem)
	if err != nil {
		return err
	}

	mounts = removeMount(mounts, mountPath)

	return writeMounts(mounts, useSystem)
}
