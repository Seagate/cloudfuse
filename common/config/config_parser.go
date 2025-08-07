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

package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/awnumar/memguard"

	"github.com/spf13/cobra"

	"github.com/fsnotify/fsnotify"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

//config is the common package to handle all configuration related functions of the entire tool
//Precedence order for retrieving config values is as follows:
//1. Flags
//2. Environment Variables
//3. Config file
//
//Any of the bind functions can be put even in init function. Calling of ReadFromConfigFile is not necessary for binding.
//Any reads must happen only after calling ReadFromConfigFile.

// ConfigChangeEventHandler is the interface that must implemented by any object that wants to be notified of changes in the config file
type ConfigChangeEventHandler interface {
	OnConfigChange()
}

type ConfigChangeEventHandlerFunc func()

func (handler ConfigChangeEventHandlerFunc) OnConfigChange() {
	handler()
}

type KeysTree map[string]interface{}

type options struct {
	path              string
	listeners         []ConfigChangeEventHandler
	flags             *pflag.FlagSet
	flagTree          *Tree
	envTree           *Tree
	completionFuncMap map[string]func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)
	secureConfig      bool
	passphrase        *memguard.Enclave
}

var userOptions options

func SetSecureConfigOptions(passphrase *memguard.Enclave) {
	userOptions.secureConfig = true
	userOptions.passphrase = passphrase
}

// SetConfigFile : set config file name to be watched by viper
func SetConfigFile(configFilePath string) {
	userOptions.path = configFilePath
	userOptions.secureConfig = false
	viper.SetConfigType("yaml")
	viper.SetConfigFile(userOptions.path)
}

// ReadFromConfigFile is used to the configFilePath and initialize viper object
func ReadFromConfigFile(configFilePath string) error {
	userOptions.path = configFilePath
	viper.SetConfigFile(userOptions.path)
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	WatchConfig()
	return nil
}

func loadConfigFromBufferToViper(configData []byte) error {
	// Set type to be yaml so that viper can parse the config data
	// and since we only allow yaml formatted config files
	viper.SetConfigType("yaml")

	err := viper.ReadConfig(strings.NewReader(string(configData)))
	if err != nil {
		return err
	}
	return nil
}

// ReadFromConfigBuffer is used to the configFilePath and initialize viper object
func ReadFromConfigBuffer(configData []byte) error {
	err := loadConfigFromBufferToViper(configData)
	if err != nil {
		return err
	}

	WatchConfig()
	return nil
}

func DecryptConfigFile(fileName string, passphrase *memguard.Enclave) error {
	cipherText, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("failed to read encrypted config file [%s]", err.Error())
	}

	if len(cipherText) == 0 {
		return fmt.Errorf("encrypted config file is empty")
	}

	plainText, err := common.DecryptData(cipherText, passphrase)
	if err != nil {
		return fmt.Errorf("failed to decrypt config file [%s]", err.Error())
	}

	err = loadConfigFromBufferToViper(plainText)
	if err != nil {
		return fmt.Errorf("failed to load decrypted config file [%s]", err.Error())
	}

	return nil
}

func WatchConfig() {
	viper.WatchConfig()
	viper.OnConfigChange(func(_ fsnotify.Event) {
		log.Crit("WatchConfig : Config change detected")
		if userOptions.secureConfig {
			err := DecryptConfigFile(userOptions.path, userOptions.passphrase)
			if err != nil {
				log.Err("WatchConfig : %s", err.Error())
				return
			}
		}
		OnConfigChange()
	})
}

func ReadConfigFromReader(reader io.Reader) error {
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(reader)
	if err != nil {
		return err
	}
	return nil
}

// AddConfigChangeEventListener function is used to register any ConfigChangeEventHandler
func AddConfigChangeEventListener(listener ConfigChangeEventHandler) {
	userOptions.listeners = append(userOptions.listeners, listener)
}

func OnConfigChange() {
	for _, listener := range userOptions.listeners {
		listener.OnConfigChange()
	}
}

// BindEnv binds the key parameter to a particular environment variable
// For a hierarchical structure pass the keys separated by a .
// For examples to access "name" field in the following structure:
//
//	auth:
//		name: value
//
// the key parameter should take on the value "auth.key"
func BindEnv(key string, envVarName string) {
	userOptions.envTree.Insert(key, envVarName)
}

// BindPFlag binds the key parameter to a particular flag
// For a hierarchical structure pass the keys separated by a .
// For examples to access "name" field in the following structure:
//
//	auth:
//		name: value
//
// the key parameter should take on the value "auth.key"
func BindPFlag(key string, flag *pflag.Flag) {
	userOptions.flagTree.Insert(key, flag)
}

//func BindPFlagWithName(key string, name string) error {
//	return viper.BindPFlag(key, userOptions.flags.Lookup(name))
//}

// UnmarshalKey is used to obtain a subtree starting from the key parameter
// For a hierarchical structure pass the keys separated by a .
// For examples to access "name" field in the following structure:
//
//	auth:
//		name: value
//
// the key parameter should take on the value "auth.key"
func UnmarshalKey(key string, obj interface{}) error {
	err := viper.UnmarshalKey(
		key,
		obj,
		func(decodeConfig *mapstructure.DecoderConfig) { decodeConfig.TagName = STRUCT_TAG },
	)
	if err != nil {
		return fmt.Errorf("config error: unmarshalling [%v]", err)
	}
	userOptions.envTree.MergeWithKey(key, obj, func(val interface{}) (interface{}, bool) {
		envVar := val.(string)
		res, ok := os.LookupEnv(envVar)
		if ok {
			return res, true
		} else {
			return "", false
		}
	})
	userOptions.flagTree.MergeWithKey(key, obj, func(val interface{}) (interface{}, bool) {
		flag := val.(*pflag.Flag)
		if flag.Changed {
			return flag.Value.String(), true
		} else {
			return "", false
		}
	})
	return nil
}

// Unmarshal populates the passed object and all the exported fields.
// use lower case attribute names to ignore a particular field
func Unmarshal(obj interface{}) error {
	err := viper.Unmarshal(
		obj,
		func(decodeConfig *mapstructure.DecoderConfig) { decodeConfig.TagName = STRUCT_TAG },
	)
	if err != nil {
		return fmt.Errorf("config error: unmarshalling [%v]", err)
	}
	userOptions.envTree.Merge(obj, func(val interface{}) (interface{}, bool) {
		envVar := val.(string)
		res, ok := os.LookupEnv(envVar)
		if ok {
			return res, true
		} else {
			return "", false
		}
	})
	userOptions.flagTree.Merge(obj, func(val interface{}) (interface{}, bool) {
		flag := val.(*pflag.Flag)
		if flag.Changed {
			return flag.Value.String(), true
		} else {
			return "", false
		}
	})

	return nil
}

func Set(key string, val string) {
	viper.Set(key, val)
}

func SetBool(key string, val bool) {
	viper.Set(key, val)
}

func IsSet(key string) bool {
	if viper.IsSet(key) {
		return true
	}
	pieces := strings.Split(key, ".")
	node := userOptions.flagTree.head
	for _, s := range pieces {
		node = node.children[s]
		if node == nil {
			return false
		}
	}
	return node.value.(*pflag.Flag).Changed
}

// AttachToFlagSet is used to attach the flags in config to the cmd flags
func AttachToFlagSet(flagset *pflag.FlagSet) {
	flagset.AddFlagSet(userOptions.flags)
}

func AttachFlagCompletions(cmd *cobra.Command) {
	for key, fn := range userOptions.completionFuncMap {
		_ = cmd.RegisterFlagCompletionFunc(key, fn)
	}
}

func RegisterFlagCompletionFunc(
	flagName string,
	completionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective),
) {
	userOptions.completionFuncMap[flagName] = completionFunc
}

func ResetConfig() {
	viper.Reset()
	userOptions = options{
		path:      "",
		listeners: make([]ConfigChangeEventHandler, 0),
		flags:     pflag.NewFlagSet("config-options", pflag.ContinueOnError),
		flagTree:  NewTree(),
		envTree:   NewTree(),
	}
}

func init() {
	userOptions.flags = pflag.NewFlagSet("config-options", pflag.ContinueOnError)

	userOptions.flagTree = NewTree()
	userOptions.envTree = NewTree()
	userOptions.completionFuncMap = make(
		map[string]func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective),
	)
}
