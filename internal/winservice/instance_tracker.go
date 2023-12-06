//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates

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
	"os"
	"path/filepath"
)

type Instance struct {
	MountPath  string `json:"mountPath"`
	ConfigFile string `json:"configFile"`
}

type Instances struct {
	Instances []Instance `json:"instances"`
}

const instanceFile = "instances.json"

func getAppDataFolder() (string, error) {
	appDataPath, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(appDataPath, "Cloudfuse")
	return fullPath, nil
}

func getInstanceTrackerFile() (string, error) {
	appDataPath, err := getAppDataFolder()
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(appDataPath, instanceFile)

	// If the file does not exist, then create it
	_, err = os.Stat(fullPath)
	if err != nil && os.IsNotExist(err) {
		_, err := os.Create(fullPath)
		if err != nil {
			return "", err
		}

		data, err := json.MarshalIndent(Instances{}, "", " ")
		if err != nil {
			return "", err
		}

		err = os.WriteFile(fullPath, data, 0644)
		if err != nil {
			return "", err
		}
	}

	return fullPath, nil
}

func readInstances(filePath string) (Instances, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return Instances{}, err
	}

	var instances Instances
	err = json.Unmarshal(file, &instances)
	return instances, err
}

func writeInstances(filePath string, instances Instances) error {
	data, err := json.MarshalIndent(instances, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

func readInstancesFromInstanceFile() ([]Instance, error) {
	instancePath, err := getInstanceTrackerFile()
	if err != nil {
		return nil, err
	}

	instances, err := readInstances(instancePath)
	if err != nil {
		return nil, err
	}

	return instances.Instances, nil
}

// AddMountJSON adds an entry to our json file with the mount path and config
// file location.
func AddMountJSON(mountPath string, configFile string) error {
	instancePath, err := getInstanceTrackerFile()
	if err != nil {
		return err
	}

	instances, err := readInstances(instancePath)
	if err != nil {
		return err
	}

	newInstance := Instance{MountPath: mountPath, ConfigFile: configFile}
	instances.Instances = append(instances.Instances, newInstance)

	return writeInstances(instancePath, instances)
}

// RemoveMountJSON removes an entry to from our json file.
func RemoveMountJSON(mountPath string) error {
	instancePath, err := getInstanceTrackerFile()
	if err != nil {
		return err
	}

	instances, err := readInstances(instancePath)
	if err != nil {
		return err
	}

	filtered := make([]Instance, 0)
	for _, instance := range instances.Instances {
		if instance.MountPath != mountPath {
			filtered = append(filtered, instance)
		}
	}

	instances.Instances = filtered
	return writeInstances(instancePath, instances)
}
