/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
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
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/spf13/cobra"
)

type configGenOptions struct {
	configFilePath   string
	outputConfigPath string
	containerName    string
	tempDirPath      string
	passphrase       string
}

var opts configGenOptions
var templatesDir = "testdata/config/"

var generateTestConfig = &cobra.Command{
	Use:               "gen-test-config",
	Short:             "Generate config file for testing given an output path.",
	Long:              "Generate config file for testing given an output path.",
	SuggestFor:        []string{"conv test config", "convert test config"},
	Hidden:            true,
	Args:              cobra.ExactArgs(0),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		var templateConfig []byte
		var err error

		if strings.Contains(opts.configFilePath, templatesDir) {
			templateConfig, err = os.ReadFile(opts.configFilePath)
		} else {
			templateConfig, err = os.ReadFile(templatesDir + opts.configFilePath)
		}

		if err != nil {
			return fmt.Errorf("failed to read file [%s]", err.Error())
		}

		// match all parameters in { }
		re := regexp.MustCompile("{.*?}")
		templateParams := re.FindAll(templateConfig, -1)
		newConfig := string(templateConfig)

		for _, param := range templateParams {
			// { 0 } -> container name
			// { 1 } -> temp path
			if string(param) == "{ 0 }" {
				re := regexp.MustCompile(string(param))
				newConfig = re.ReplaceAllString(newConfig, opts.containerName)
			} else if string(param) == "{ 1 }" {
				re := regexp.MustCompile(string(param))
				newConfig = re.ReplaceAllString(newConfig, opts.tempDirPath)
			} else {
				envVar := os.Getenv(string(param)[2 : len(string(param))-2])
				re := regexp.MustCompile(string(param))
				newConfig = re.ReplaceAllString(newConfig, envVar)
			}
		}

		// write the config with the params to the output file
		err = os.WriteFile(opts.outputConfigPath, []byte(newConfig), 0700)
		if err != nil {
			return fmt.Errorf("failed to write file [%s]", err.Error())
		}

		return nil
	},
}

// Command used by plugins to generate encrypted config file based on a provided template.
var generateConfig = &cobra.Command{
	Use:               "gen-config",
	Short:             "Generate encrypted config file based on template.",
	Long:              "Generate encrypted config file based on template.",
	SuggestFor:        []string{"gen-config"},
	Hidden:            true,
	Args:              cobra.ExactArgs(0),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		var templateConfig []byte
		var err error

		templateConfig, err = os.ReadFile(opts.configFilePath)
		if err != nil {
			return fmt.Errorf("failed to read file [%s]", err.Error())
		}

		// match all parameters in { }
		re := regexp.MustCompile("{.*?}")
		templateParams := re.FindAll(templateConfig, -1)
		newConfig := string(templateConfig)

		for _, param := range templateParams {
			// { 0 } -> temp path
			if string(param) == "{ 0 }" {
				re := regexp.MustCompile(string(param))
				newConfig = re.ReplaceAllString(newConfig, opts.tempDirPath)
			} else {
				envVar := os.Getenv(string(param)[2 : len(string(param))-2])
				re := regexp.MustCompile(string(param))
				newConfig = re.ReplaceAllString(newConfig, envVar)
			}
		}

		cipherText, err := common.EncryptData([]byte(newConfig), []byte(opts.passphrase))
		if err != nil {
			return err
		}

		// write the config with the params to the output file
		err = os.WriteFile(opts.outputConfigPath, cipherText, 0700)
		if err != nil {
			return fmt.Errorf("failed to write file [%s]", err.Error())
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateTestConfig)
	generateTestConfig.Flags().StringVar(&opts.configFilePath, "config-file", "", "Input config file.")
	generateTestConfig.Flags().StringVar(&opts.outputConfigPath, "output-file", "", "Output config file path.")
	generateTestConfig.Flags().StringVar(&opts.containerName, "container-name", "", "Container name.")
	generateTestConfig.Flags().StringVar(&opts.tempDirPath, "temp-path", "", "Temporary file path.")

	rootCmd.AddCommand(generateConfig)
	generateConfig.Flags().StringVar(&opts.configFilePath, "config-file", "", "Input config file.")
	generateConfig.Flags().StringVar(&opts.outputConfigPath, "output-file", "", "Output config file path.")
	generateConfig.Flags().StringVar(&opts.tempDirPath, "temp-path", "", "Temporary file path.")
	generateConfig.Flags().StringVar(&opts.passphrase, "passphrase", "",
		"Key to be used for encryption / decryption. Key length shall be 16 (AES-128), 24 (AES-192), or 32 (AES-256) bytes in length.")
}
