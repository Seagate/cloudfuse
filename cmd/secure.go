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
	"path/filepath"

	"github.com/Seagate/cloudfuse/common"
	"github.com/awnumar/memguard"

	"github.com/spf13/cobra"
)

type secureOptions struct {
	Operation  string
	ConfigFile string
	PassPhrase string
	OutputFile string
	Key        string
	Value      string
}

const SecureConfigEnvName string = "CLOUDFUSE_SECURE_CONFIG_PASSPHRASE"
const SecureConfigExtension string = ".aes"

var secOpts secureOptions
var encryptedPassphrase *memguard.Enclave

// Section defining all the command that we have in secure feature
var secureCmd = &cobra.Command{
	Use:        "secure",
	Short:      "Encrypt / Decrypt your config file",
	Long:       "Encrypt or decrypt configuration files containing sensitive credentials.\nEncrypted config files use the .aes extension.",
	Aliases:    []string{"sec"},
	SuggestFor: []string{"secre", "encrypt", "decrypt"},
	GroupID:    groupConfig,
	Example: `  # Encrypt a config file
  cloudfuse secure encrypt -c config.yaml -p SECRET

  # Decrypt a config file
  cloudfuse secure decrypt -c config.yaml.aes -p SECRET

  # Get a key from encrypted config
  cloudfuse secure get -c config.yaml.aes -p SECRET -k azstorage.account-name`,
	// PersistentPreRunE validates options for all subcommands
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return validateOptions()
	},
}

var encryptCmd = &cobra.Command{
	Use:        "encrypt",
	Short:      "Encrypt your config file",
	Long:       "Encrypt a YAML configuration file using AES encryption.\nThe output file will have a .aes extension.",
	SuggestFor: []string{"en", "enc"},
	Example: `  # Encrypt config file (creates config.yaml.aes)
  cloudfuse secure encrypt -c config.yaml -p SECRET

  # Encrypt to a specific output file
  cloudfuse secure encrypt -c config.yaml -p SECRET -o secure.aes`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validation handled by PersistentPreRunE
		_, err := encryptConfigFile(true)
		if err != nil {
			return fmt.Errorf("failed to encrypt config file: %w", err)
		}

		return nil
	},
}

var decryptCmd = &cobra.Command{
	Use:        "decrypt",
	Short:      "Decrypt your config file",
	Long:       "Decrypt an AES-encrypted configuration file back to plain YAML.",
	SuggestFor: []string{"de", "dec"},
	Example: `  # Decrypt config file
  cloudfuse secure decrypt -c config.yaml.aes -p SECRET

  # Decrypt to a specific output file
  cloudfuse secure decrypt -c config.yaml.aes -p SECRET -o config.yaml`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validation handled by PersistentPreRunE
		_, err := decryptConfigFile(true)
		if err != nil {
			return fmt.Errorf("failed to decrypt config file: %w", err)
		}

		return nil
	},
}

//--------------- command section ends

func validateOptions() error {
	if secOpts.PassPhrase == "" {
		secOpts.PassPhrase = os.Getenv(SecureConfigEnvName)
		if secOpts.PassPhrase == "" {
			return errors.New(
				"provide the passphrase as a cli parameter or configure the CLOUDFUSE_SECURE_CONFIG_PASSPHRASE environment variable",
			)
		}
	}

	encryptedPassphrase = memguard.NewEnclave([]byte(secOpts.PassPhrase))

	if secOpts.ConfigFile == "" {
		return errors.New("config file not provided, check usage")
	}

	if _, err := os.Stat(secOpts.ConfigFile); os.IsNotExist(err) {
		return errors.New("config file does not exist")
	}

	return nil
}

// encryptConfigFile: Encrypt config file using the passphrase provided by user
func encryptConfigFile(saveConfig bool) ([]byte, error) {
	plaintext, err := os.ReadFile(secOpts.ConfigFile)
	if err != nil {
		return nil, err
	}

	cipherText, err := common.EncryptData(plaintext, encryptedPassphrase)
	if err != nil {
		return nil, err
	}

	if saveConfig {
		outputFileName := ""
		if secOpts.OutputFile == "" {
			outputFileName = secOpts.ConfigFile + SecureConfigExtension
		} else {
			outputFileName = secOpts.OutputFile
		}

		return cipherText, saveToFile(outputFileName, cipherText, true)
	}

	return cipherText, nil
}

// decryptConfigFile: Decrypt config file using the passphrase provided by user
func decryptConfigFile(saveConfig bool) ([]byte, error) {
	cipherText, err := os.ReadFile(secOpts.ConfigFile)
	if err != nil {
		return nil, err
	}

	plainText, err := common.DecryptData(cipherText, encryptedPassphrase)
	if err != nil {
		return nil, err
	}

	if saveConfig {
		outputFileName := ""
		if secOpts.OutputFile == "" {
			outputFileName = secOpts.ConfigFile
			extension := filepath.Ext(outputFileName)
			outputFileName = outputFileName[0 : len(outputFileName)-len(extension)]
		} else {
			outputFileName = secOpts.OutputFile
		}

		return plainText, saveToFile(outputFileName, plainText, true)
	}

	return plainText, nil
}

// saveToFile: Save the newly generated config file and delete the source if requested
func saveToFile(configFileName string, data []byte, deleteSource bool) error {
	err := os.WriteFile(configFileName, data, 0644)
	if err != nil {
		return err
	}

	if deleteSource {
		// Delete the original file as we now have a encrypted config file
		err = os.Remove(secOpts.ConfigFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(secureCmd)
	secureCmd.AddCommand(encryptCmd)
	secureCmd.AddCommand(decryptCmd)
	// Flags that needs to be accessible at all subcommand level shall be defined in persistentflags only
	secureCmd.PersistentFlags().StringVarP(&secOpts.ConfigFile, "config-file", "c", "",
		"Configuration file to be encrypted / decrypted")
	_ = secureCmd.MarkPersistentFlagFilename("config-file", "yaml", "aes")
	_ = secureCmd.MarkPersistentFlagRequired("config-file")
	_ = secureCmd.RegisterFlagCompletionFunc(
		"config-file",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return []string{"yaml", "yml", "aes"}, cobra.ShellCompDirectiveFilterFileExt
		},
	)

	secureCmd.PersistentFlags().StringVarP(&secOpts.PassPhrase, "passphrase", "p", "",
		"Password to decrypt config file. Can also be specified by env-variable CLOUDFUSE_SECURE_CONFIG_PASSPHRASE.")

	secureCmd.PersistentFlags().StringVarP(&secOpts.OutputFile, "output-file", "o", "",
		"Path and name for the output file")
	_ = secureCmd.MarkPersistentFlagFilename("output-file", "yaml", "aes")
	_ = secureCmd.RegisterFlagCompletionFunc(
		"output-file",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return []string{"yaml", "yml", "aes"}, cobra.ShellCompDirectiveFilterFileExt
		},
	)
}
