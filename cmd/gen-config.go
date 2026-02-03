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
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/Seagate/cloudfuse/common"
	"github.com/awnumar/memguard"
	"github.com/spf13/cobra"
)

type genConfigParams struct {
	configFilePath   string
	outputConfigPath string
	tempDirPath      string
	passphrase       string
}

var optsGenCfg genConfigParams

var generatedConfig = &cobra.Command{
	Use:        "gen-config",
	Short:      "Generate config file from template.",
	Long:       "Generate a cloudfuse configuration file from a template.\nReplaces placeholder values with provided parameters.",
	SuggestFor: []string{"generate default config", "generate config"},
	Hidden:     true,
	Args:       cobra.ExactArgs(0),
	Example: `  # Generate config from template
  cloudfuse gen-config --config-file=template.yaml --output-file=config.yaml --temp-path=/tmp/cloudfuse`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

	encryptedPassphrase = memguard.NewEnclave([]byte(optsGenCfg.passphrase))

	return nil
}

func init() {
	rootCmd.AddCommand(generatedConfig)

	generatedConfig.Flags().
		StringVar(&optsGenCfg.configFilePath, "config-file", "", "Input config file.")
	generatedConfig.Flags().
		StringVar(&optsGenCfg.outputConfigPath, "output-file", "", "Output config file path.")
	generatedConfig.Flags().
		StringVar(&optsGenCfg.tempDirPath, "temp-path", "", "Temporary file path.")
	generatedConfig.Flags().StringVar(&optsGenCfg.passphrase, "passphrase", "",
		"Key to be used for encryption / decryption. Key length shall be 16 (AES-128), 24 (AES-192), or 32 (AES-256) bytes in length.")
}
