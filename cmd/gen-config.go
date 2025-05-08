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
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/awnumar/memguard"
	"github.com/spf13/cobra"
)

type genConfigParams struct {
	blockCache       bool   `config:"block-cache" yaml:"block-cache,omitempty"`
	directIO         bool   `config:"direct-io"   yaml:"direct-io,omitempty"`
	readOnly         bool   `config:"ro"          yaml:"ro,omitempty"`
	tmpPath          string `config:"tmp-path"    yaml:"tmp-path,omitempty"`
	outputFile       string `config:"o"           yaml:"o,omitempty"`
	configFilePath   string
	outputConfigPath string
	tempDirPath      string
	passphrase       string
}

var optsGenCfg genConfigParams

var generatedConfig = &cobra.Command{
	Use:               "gen-config",
	Short:             "Generate default config file.",
	Long:              "Generate default config file with the values pre-caculated by cloudfuse.",
	SuggestFor:        []string{"generate default config", "generate config"},
	Hidden:            true,
	Args:              cobra.ExactArgs(0),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		// If the user passes a config file path, then use that as a template to generate a config
		if optsGenCfg.configFilePath != "" {
			var templateConfig []byte

			err := validateGenConfigOptions()
			if err != nil {
				return fmt.Errorf("failed to validate options [%s]", err.Error())
			}

			encryptedPassphrase = memguard.NewEnclave([]byte(optsGenCfg.passphrase))

			templateConfig, err = os.ReadFile(optsGenCfg.configFilePath)
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
					newConfig = re.ReplaceAllString(newConfig, optsGenCfg.tempDirPath)
				} else {
					envVar := os.Getenv(string(param)[2 : len(string(param))-2])
					re := regexp.MustCompile(string(param))
					newConfig = re.ReplaceAllString(newConfig, envVar)
				}
			}

			cipherText, err := common.EncryptData([]byte(newConfig), encryptedPassphrase)
			if err != nil {
				return err
			}

			// write the config with the params to the output file
			err = os.WriteFile(optsGenCfg.outputConfigPath, cipherText, 0700)
			if err != nil {
				return fmt.Errorf("failed to write file [%s]", err.Error())
			}

			return nil
		}

		// Check if configTmp is not provided when component is fc
		if (!optsGenCfg.blockCache) && optsGenCfg.tmpPath == "" {
			return fmt.Errorf(
				"temp path is required for file cache mode. Use flag --tmp-path to provide the path",
			)
		}

		// Set the configs
		if optsGenCfg.readOnly {
			config.Set("read-only", "true")
		}

		if optsGenCfg.directIO {
			config.Set("direct-io", "true")
		}

		config.Set("tmp-path", optsGenCfg.tmpPath)

		// Create the pipeline
		pipeline := []string{"libfuse"}
		if optsGenCfg.blockCache {
			pipeline = append(pipeline, "block_cache")
		} else {
			pipeline = append(pipeline, "file_cache")
		}

		if !optsGenCfg.directIO {
			pipeline = append(pipeline, "attr_cache")
		}
		pipeline = append(pipeline, "azstorage")

		var sb strings.Builder

		if optsGenCfg.directIO {
			sb.WriteString("direct-io: true\n")
		}

		if optsGenCfg.readOnly {
			sb.WriteString("read-only: true\n\n")
		}

		sb.WriteString(
			"# Logger configuration\n#logging:\n  #  type: syslog|silent|base\n  #  level: log_off|log_crit|log_err|log_warning|log_info|log_trace|log_debug\n",
		)
		sb.WriteString(
			"  #  file-path: <path where log files shall be stored. Default - '$HOME/.cloudfuse/cloudfuse.log'>\n",
		)

		sb.WriteString("\ncomponents:\n")
		for _, component := range pipeline {
			sb.WriteString(fmt.Sprintf("  - %s\n", component))
		}

		for _, component := range pipeline {
			c := internal.GetComponent(component)
			if c == nil {
				return fmt.Errorf("generatedConfig:: error getting component [%s]", component)
			}
			sb.WriteString("\n")
			sb.WriteString(c.GenConfig())
		}

		sb.WriteString(
			"\n#Required\n#azstorage:\n  #  type: block|adls \n  #  account-name: <name of the storage account>\n  #  container: <name of the storage container to be mounted>\n  #  endpoint: <example - https://account-name.blob.core.windows.net>\n  ",
		)
		sb.WriteString(
			"#  mode: key|sas|spn|msi|azcli \n  #  account-key: <storage account key>\n  # OR\n  #  sas: <storage account sas>\n  # OR\n  #  appid: <storage account app id / client id for MSI>\n  # OR\n  #  tenantid: <storage account tenant id for SPN",
		)

		filePath := ""
		if optsGenCfg.outputFile == "" {
			filePath = "./cloudfuse.yaml"
		} else {
			filePath = optsGenCfg.outputFile
		}

		var err error = nil
		if optsGenCfg.outputFile == "console" {
			fmt.Println(sb.String())
		} else {
			err = common.WriteToFile(filePath, sb.String(), common.WriteToFileOptions{Flags: os.O_TRUNC, Permission: 0644})
		}

		return err
	},
}

func validateGenConfigOptions() error {
	if optsGenCfg.passphrase == "" {
		optsGenCfg.passphrase = os.Getenv(SecureConfigEnvName)
		if optsGenCfg.passphrase == "" {
			return errors.New(
				"provide the passphrase as a cli parameter or configure the CLOUDFUSE_SECURE_CONFIG_PASSPHRASE environment variable",
			)
		}
	}

	_, err := base64.StdEncoding.DecodeString(string(optsGenCfg.passphrase))
	if err != nil {
		return fmt.Errorf("passphrase is not valid base64 encoded [%s]", err.Error())
	}

	encryptedPassphrase = memguard.NewEnclave([]byte(optsGenCfg.passphrase))

	return nil
}

func init() {
	rootCmd.AddCommand(generatedConfig)

	generatedConfig.Flags().
		BoolVar(&optsGenCfg.blockCache, "block-cache", false, "Block-Cache shall be used as caching strategy")
	generatedConfig.Flags().
		BoolVar(&optsGenCfg.directIO, "direct-io", false, "Direct-io mode shall be used")
	generatedConfig.Flags().BoolVar(&optsGenCfg.readOnly, "ro", false, "Mount in read-only mode")
	generatedConfig.Flags().
		StringVar(&optsGenCfg.tmpPath, "tmp-path", "", "Temp cache path to be used")
	generatedConfig.Flags().StringVar(&optsGenCfg.outputFile, "o", "", "Output file location")

	generatedConfig.Flags().
		StringVar(&optsGenCfg.configFilePath, "config-file", "", "Input config file.")
	generatedConfig.Flags().
		StringVar(&optsGenCfg.outputConfigPath, "output-file", "", "Output config file path.")
	generatedConfig.Flags().
		StringVar(&optsGenCfg.tempDirPath, "temp-path", "", "Temporary file path.")
	generatedConfig.Flags().StringVar(&optsGenCfg.passphrase, "passphrase", "",
		"Key to be used for encryption / decryption. Key length shall be 16 (AES-128), 24 (AES-192), or 32 (AES-256) bytes in length.")
}
