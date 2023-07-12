/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

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
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"lyvecloudfuse/common"
	"lyvecloudfuse/common/config"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal"

	"github.com/sevlyar/go-daemon"
	"github.com/spf13/cobra"
)

type LogOptions struct {
	Type           string `config:"type" yaml:"type,omitempty"`
	LogLevel       string `config:"level" yaml:"level,omitempty"`
	LogFilePath    string `config:"file-path" yaml:"file-path,omitempty"`
	MaxLogFileSize uint64 `config:"max-file-size-mb" yaml:"max-file-size-mb,omitempty"`
	LogFileCount   uint64 `config:"file-count" yaml:"file-count,omitempty"`
	TimeTracker    bool   `config:"track-time" yaml:"track-time,omitempty"`
}

type mountOptions struct {
	MountPath  string
	ConfigFile string

	Logging           LogOptions     `config:"logging"`
	Components        []string       `config:"components"`
	Foreground        bool           `config:"foreground"`
	NonEmpty          bool           `config:"nonempty"`
	DefaultWorkingDir string         `config:"default-working-dir"`
	CPUProfile        string         `config:"cpu-profile"`
	MemProfile        string         `config:"mem-profile"`
	PassPhrase        string         `config:"passphrase"`
	SecureConfig      bool           `config:"secure-config"`
	DynamicProfiler   bool           `config:"dynamic-profile"`
	ProfilerPort      int            `config:"profiler-port"`
	ProfilerIP        string         `config:"profiler-ip"`
	MonitorOpt        monitorOptions `config:"health_monitor"`
	WaitForMount      time.Duration  `config:"wait-for-mount"`

	// v1 support
	Streaming      bool     `config:"streaming"`
	AttrCache      bool     `config:"use-attr-cache"`
	LibfuseOptions []string `config:"libfuse-options"`
}

var options mountOptions

func (opt *mountOptions) validate(skipEmptyMount bool) error {
	if opt.MountPath == "" {
		return fmt.Errorf("mount path not provided")
	}

	// Windows requires that the mount directory does not exist while
	// linux requires that the directory does exist. So we skip these
	// checks if
	if runtime.GOOS == "windows" {
		_, err := os.Stat(opt.MountPath)
		if !os.IsNotExist(err) {
			return fmt.Errorf("mount directory already exists")
		}
	} else {
		if _, err := os.Stat(opt.MountPath); os.IsNotExist(err) {
			return fmt.Errorf("mount directory does not exists")
		} else if !skipEmptyMount && !common.IsDirectoryEmpty(opt.MountPath) {
			return fmt.Errorf("mount directory is not empty")
		}
	}

	if common.IsDirectoryMounted(opt.MountPath) {
		return fmt.Errorf("directory is already mounted")
	}

	if err := common.ELogLevel.Parse(opt.Logging.LogLevel); err != nil {
		return fmt.Errorf("invalid log level [%s]", err.Error())
	}

	if opt.DefaultWorkingDir != "" {
		common.DefaultWorkDir = opt.DefaultWorkingDir

		if opt.Logging.LogFilePath == common.DefaultLogFilePath {
			// If default-working-dir is set then default log path shall be set to that path
			// Ignore if specific log-path is provided by user
			opt.Logging.LogFilePath = common.JoinUnixFilepath(common.DefaultWorkDir, "lyvecloudfuse.log")
		}

		common.DefaultLogFilePath = common.JoinUnixFilepath(common.DefaultWorkDir, "lyvecloudfuse.log")
	}

	f, err := os.Stat(common.ExpandPath(common.DefaultWorkDir))
	if err == nil && !f.IsDir() {
		return fmt.Errorf("default work dir '%s' is not a directory", common.DefaultWorkDir)
	}

	if err != nil && os.IsNotExist(err) {
		// create the default work dir
		if err = os.MkdirAll(common.ExpandPath(common.DefaultWorkDir), 0777); err != nil {
			return fmt.Errorf("failed to create default work dir [%s]", err.Error())
		}
	}

	opt.Logging.LogFilePath = common.ExpandPath(opt.Logging.LogFilePath)
	if !common.DirectoryExists(filepath.Dir(opt.Logging.LogFilePath)) {
		err := os.MkdirAll(filepath.Dir(opt.Logging.LogFilePath), os.FileMode(0776)|os.ModeDir)
		if err != nil {
			return fmt.Errorf("invalid log file path [%s]", err.Error())
		}
	}

	// A user provided value of 0 doesn't make sense for MaxLogFileSize or LogFileCount.
	if opt.Logging.MaxLogFileSize == 0 {
		opt.Logging.MaxLogFileSize = common.DefaultMaxLogFileSize
	}

	if opt.Logging.LogFileCount == 0 {
		opt.Logging.LogFileCount = common.DefaultLogFileCount
	}

	return nil
}

func OnConfigChange() {
	newLogOptions := &LogOptions{}
	err := config.UnmarshalKey("logging", newLogOptions)
	if err != nil {
		log.Err("Mount::OnConfigChange : Invalid logging options [%s]", err.Error())
	}

	var logLevel common.LogLevel
	err = logLevel.Parse(newLogOptions.LogLevel)
	if err != nil {
		log.Err("Mount::OnConfigChange : Invalid log level [%s]", newLogOptions.LogLevel)
	}

	err = log.SetConfig(common.LogConfig{
		Level:       logLevel,
		FilePath:    common.ExpandPath(newLogOptions.LogFilePath),
		MaxFileSize: newLogOptions.MaxLogFileSize,
		FileCount:   newLogOptions.LogFileCount,
		TimeTracker: newLogOptions.TimeTracker,
	})

	if err != nil {
		log.Err("Mount::OnConfigChange : Unable to reset Logging options [%s]", err.Error())
	}
}

// parseConfig : Based on config file or encrypted data parse the provided config
func parseConfig() error {
	options.ConfigFile = common.ExpandPath(options.ConfigFile)

	// Based on extension decide file is encrypted or not
	if options.SecureConfig ||
		filepath.Ext(options.ConfigFile) == SecureConfigExtension {

		// Validate config is to be secured on write or not
		if options.PassPhrase == "" {
			options.PassPhrase = os.Getenv(SecureConfigEnvName)
		}

		if options.PassPhrase == "" {
			return fmt.Errorf("no passphrase provided to decrypt the config file.\n Either use --passphrase cli option or store passphrase in LYVECLOUDFUSE_SECURE_CONFIG_PASSPHRASE environment variable")
		}

		cipherText, err := os.ReadFile(options.ConfigFile)
		if err != nil {
			return fmt.Errorf("failed to read encrypted config file %s [%s]", options.ConfigFile, err.Error())
		}

		plainText, err := common.DecryptData(cipherText, []byte(options.PassPhrase))
		if err != nil {
			return fmt.Errorf("failed to decrypt config file %s [%s]", options.ConfigFile, err.Error())
		}

		config.SetConfigFile(options.ConfigFile)
		config.SetSecureConfigOptions(options.PassPhrase)
		err = config.ReadFromConfigBuffer(plainText)
		if err != nil {
			return fmt.Errorf("invalid decrypted config file [%s]", err.Error())
		}

	} else {
		err := config.ReadFromConfigFile(options.ConfigFile)
		if err != nil {
			return fmt.Errorf("invalid config file [%s]", err.Error())
		}
	}

	return nil
}

// We use the cobra library to provide a CLI for Lyve Cloud FUSE.
// Look at https://cobra.dev/ for more information
var mountCmd = &cobra.Command{
	Use:               "mount [path]",
	Short:             "Mounts the azure container as a filesystem",
	Long:              "Mounts the azure container as a filesystem",
	SuggestFor:        []string{"mnt", "mout"},
	Args:              cobra.ExactArgs(1),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(_ *cobra.Command, args []string) error {
		options.MountPath = common.ExpandPath(args[0])
		configFileExists := true

		if options.ConfigFile == "" {
			// Config file is not set in cli parameters
			// Lyvecloudfuse defaults to config.yaml in current directory
			// If the file does not exists then user might have configured required things in env variables
			// Fall back to defaults and let components fail if all required env variables are not set.
			_, err := os.Stat(common.DefaultConfigFilePath)
			if err != nil && os.IsNotExist(err) {
				configFileExists = false
			} else {
				options.ConfigFile = common.DefaultConfigFilePath
			}
		}

		if configFileExists {
			err := parseConfig()
			if err != nil {
				return err
			}
		}

		err := config.Unmarshal(&options)
		if err != nil {
			return fmt.Errorf("failed to unmarshal config [%s]", err.Error())
		}

		if !configFileExists || len(options.Components) == 0 {
			pipeline := []string{"libfuse"}

			if config.IsSet("streaming") && options.Streaming {
				pipeline = append(pipeline, "stream")
			} else {
				pipeline = append(pipeline, "file_cache")
			}

			// by default attr-cache is enable in v2
			// only way to disable is to pass cli param and set it to false
			if options.AttrCache {
				pipeline = append(pipeline, "attr_cache")
			}

			pipeline = append(pipeline, "azstorage")
			options.Components = pipeline
		}

		skipNonEmpty := false

		if config.IsSet("libfuse-options") {
			for _, v := range options.LibfuseOptions {
				parameter := strings.Split(v, "=")
				if len(parameter) > 2 || len(parameter) <= 0 {
					return errors.New(common.FuseAllowedFlags)
				}

				v = strings.TrimSpace(v)
				if ignoreFuseOptions(v) {
					continue
				} else if v == "allow_other" || v == "allow_other=true" {
					config.Set("allow-other", "true")
				} else if strings.HasPrefix(v, "attr_timeout=") {
					config.Set("libfuse.attribute-expiration-sec", parameter[1])
				} else if strings.HasPrefix(v, "entry_timeout=") {
					config.Set("libfuse.entry-expiration-sec", parameter[1])
				} else if strings.HasPrefix(v, "negative_timeout=") {
					config.Set("libfuse.negative-entry-expiration-sec", parameter[1])
				} else if v == "ro" || v == "ro=true" {
					config.Set("read-only", "true")
				} else if v == "allow_root" || v == "allow_root=true" {
					config.Set("allow-root", "true")
					// config.Set("libfuse.default-permission", "700")
				} else if v == "nonempty" {
					skipNonEmpty = true
					config.Set("nonempty", "true")
				} else if strings.HasPrefix(v, "umask=") {
					permission, err := strconv.ParseUint(parameter[1], 10, 32)
					if err != nil {
						return fmt.Errorf("failed to parse umask [%s]", err.Error())
					}
					perm := ^uint32(permission) & 777
					config.Set("libfuse.default-permission", fmt.Sprint(perm))
				} else if strings.HasPrefix(v, "uid=") {
					val, err := strconv.ParseUint(parameter[1], 10, 32)
					if err != nil {
						return fmt.Errorf("failed to parse uid [%s]", err.Error())
					}
					config.Set("libfuse.uid", fmt.Sprint(val))
				} else if strings.HasPrefix(v, "gid=") {
					val, err := strconv.ParseUint(parameter[1], 10, 32)
					if err != nil {
						return fmt.Errorf("failed to parse gid [%s]", err.Error())
					}
					config.Set("libfuse.gid", fmt.Sprint(val))
				} else if v == "direct_io" || v == "direct_io=true" {
					config.Set("libfuse.direct-io", "true")
				} else {
					return errors.New(common.FuseAllowedFlags)
				}
			}
		}

		if !config.IsSet("logging.file-path") {
			options.Logging.LogFilePath = common.DefaultLogFilePath
		}

		if !config.IsSet("logging.level") {
			options.Logging.LogLevel = "LOG_WARNING"
		}

		err = options.validate(options.NonEmpty || skipNonEmpty)
		if err != nil {
			return err
		}

		var logLevel common.LogLevel
		err = logLevel.Parse(options.Logging.LogLevel)
		if err != nil {
			return fmt.Errorf("invalid log level [%s]", err.Error())
		}

		err = log.SetDefaultLogger(options.Logging.Type, common.LogConfig{
			FilePath:    options.Logging.LogFilePath,
			MaxFileSize: options.Logging.MaxLogFileSize,
			FileCount:   options.Logging.LogFileCount,
			Level:       logLevel,
			TimeTracker: options.Logging.TimeTracker,
		})

		if err != nil {
			return fmt.Errorf("failed to initialize logger [%s]", err.Error())
		}

		if !disableVersionCheck {
			err := VersionCheck()
			if err != nil {
				log.Err(err.Error())
			}
		}

		if config.IsSet("invalidate-on-sync") {
			log.Warn("mount: unsupported v1 CLI parameter: invalidate-on-sync is always true in lyvecloudfuse.")
		}
		if config.IsSet("pre-mount-validate") {
			log.Warn("mount: unsupported v1 CLI parameter: pre-mount-validate is always true in lyvecloudfuse.")
		}
		if config.IsSet("basic-remount-check") {
			log.Warn("mount: unsupported v1 CLI parameter: basic-remount-check is always true in lyvecloudfuse.")
		}

		common.EnableMonitoring = options.MonitorOpt.EnableMon

		// check if blobfuse stats monitor is added in the disable list
		for _, mon := range options.MonitorOpt.DisableList {
			if mon == common.BfuseStats {
				common.BfsDisabled = true
				break
			}
		}

		config.Set("mount-path", options.MountPath)

		var pipeline *internal.Pipeline

		log.Crit("Starting Lyvecloudfuse Mount : %s on [%s]", common.LyvecloudfuseVersion, common.GetCurrentDistro())
		log.Crit("Logging level set to : %s", logLevel.String())

		// If on Linux start with the go deamon
		// If on Windows, don't use the daemon since it is not supported
		// TODO: Enable running as a service on Windows
		if runtime.GOOS == "windows" {
			pipeline, err = internal.NewPipeline(options.Components, true)
		} else {
			pipeline, err = internal.NewPipeline(options.Components, !daemon.WasReborn())
		}
		if err != nil {
			log.Err("mount : failed to initialize new pipeline [%v]", err)
			return Destroy(fmt.Sprintf("failed to initialize new pipeline [%s]", err.Error()))
		}

		common.ForegroundMount = options.Foreground

		log.Info("mount: Mounting lyvecloudfuse on %s", options.MountPath)
		// Prevent mounting in background on Windows
		// TODO: Enable background support using windows services
		if !options.Foreground && runtime.GOOS != "windows" {
			pidFile := strings.Replace(options.MountPath, "/", "_", -1) + ".pid"
			pidFileName := filepath.Join(os.ExpandEnv(common.DefaultWorkDir), pidFile)

			pid := os.Getpid()
			fname := fmt.Sprintf("/tmp/lyvecloudfuse.%v", pid)

			ctx, _ := context.WithCancel(context.Background()) //nolint
			err := createDaemon(pipeline, ctx, pidFileName, 0644, 027, fname)
			if err != nil {
				log.Err("mount: failed to create daemon [%v]", err.Error())
			}
		} else {
			if options.CPUProfile != "" {
				os.Remove(options.CPUProfile)
				f, err := os.Create(options.CPUProfile)
				if err != nil {
					fmt.Printf("Error opening file for cpuprofile [%s]", err.Error())
				}
				defer f.Close()
				if err := pprof.StartCPUProfile(f); err != nil {
					fmt.Printf("Failed to start cpuprofile [%s]", err.Error())
				}
				defer pprof.StopCPUProfile()
			}

			setGOConfig()
			go startDynamicProfiler()

			log.Debug("mount: foreground enabled")
			err = runPipeline(pipeline, context.Background())
			if err != nil {
				return err
			}

			if options.MemProfile != "" {
				os.Remove(options.MemProfile)
				f, err := os.Create(options.MemProfile)
				if err != nil {
					fmt.Printf("Error opening file for memprofile [%s]", err.Error())
				}
				defer f.Close()
				runtime.GC()
				if err = pprof.WriteHeapProfile(f); err != nil {
					fmt.Printf("Error memory profiling [%s]", err.Error())
				}
			}
		}
		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	},
}

func ignoreFuseOptions(opt string) bool {
	for _, o := range common.FuseIgnoredFlags() {
		// Flags like uid and gid come with value so exact string match is not correct in that case.
		if strings.HasPrefix(opt, o) {
			return true
		}
	}
	return false
}

func runPipeline(pipeline *internal.Pipeline, ctx context.Context) error {
	pid := fmt.Sprintf("%v", os.Getpid())
	common.TransferPipe += "_" + pid
	common.PollingPipe += "_" + pid
	log.Debug("Mount::runPipeline : lyvecloudfuse pid = %v, transfer pipe = %v, polling pipe = %v", pid, common.TransferPipe, common.PollingPipe)

	go startMonitor(os.Getpid())

	err := pipeline.Start(ctx)
	if err != nil {
		log.Err("mount: error unable to start pipeline [%s]", err.Error())
		return Destroy(fmt.Sprintf("unable to start pipeline [%s]", err.Error()))
	}

	err = pipeline.Stop()
	if err != nil {
		log.Err("mount: error unable to stop pipeline [%s]", err.Error())
		return Destroy(fmt.Sprintf("unable to stop pipeline [%s]", err.Error()))
	}

	_ = log.Destroy()
	return nil
}

func startMonitor(pid int) {
	if common.EnableMonitoring {
		log.Debug("Mount::startMonitor : pid = %v, config-file = %v", pid, options.ConfigFile)
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"health-monitor", fmt.Sprintf("--pid=%v", pid), fmt.Sprintf("--config-file=%s", options.ConfigFile)})
		err := rootCmd.Execute()
		if err != nil {
			common.EnableMonitoring = false
			log.Err("Mount::startMonitor : [%s]", err.Error())
		}
	}
}

func setGOConfig() {
	// Ensure we always have more than 1 OS thread running goroutines, since there are issues with having just 1.
	isOnlyOne := runtime.GOMAXPROCS(0) == 1
	if isOnlyOne {
		runtime.GOMAXPROCS(2)
	}

	// Golang's default behaviour is to GC when new objects = (100% of) total of objects surviving previous GC.
	// Set it to lower level so that memory if freed up early
	debug.SetGCPercent(80)
}

func startDynamicProfiler() {
	if !options.DynamicProfiler {
		return
	}

	if options.ProfilerIP == "" {
		// By default enable profiler on 127.0.0.1
		options.ProfilerIP = "localhost"
	}

	if options.ProfilerPort == 0 {
		// This is default go profiler port
		options.ProfilerPort = 6060
	}

	connStr := fmt.Sprintf("%s:%d", options.ProfilerIP, options.ProfilerPort)
	log.Info("Mount::startDynamicProfiler : Staring profiler on [%s]", connStr)

	// To check dynamic profiling info http://<ip>:<port>/debug/pprof
	// for e.g. for default config use http://localhost:6060/debug/pprof
	// Also CLI based profiler can be used
	// e.g. go tool pprof http://localhost:6060/debug/pprof/heap
	//      go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
	//      go tool pprof http://localhost:6060/debug/pprof/block
	//
	err := http.ListenAndServe(connStr, nil)
	if err != nil {
		log.Err("Mount::startDynamicProfiler : Failed to start dynamic profiler [%s]", err.Error())
	}
}

func init() {
	rootCmd.AddCommand(mountCmd)

	options = mountOptions{}

	mountCmd.AddCommand(mountListCmd)
	mountCmd.AddCommand(mountAllCmd)

	mountCmd.PersistentFlags().StringVar(&options.ConfigFile, "config-file", "",
		"Configures the path for the file where the account credentials are provided. Default is config.yaml in current directory.")
	_ = mountCmd.MarkPersistentFlagFilename("config-file", "yaml")

	mountCmd.PersistentFlags().BoolVar(&options.SecureConfig, "secure-config", false,
		"Encrypt auto generated config file for each container")

	mountCmd.PersistentFlags().StringVar(&options.PassPhrase, "passphrase", "",
		"Key to decrypt config file. Can also be specified by env-variable LYVECLOUDFUSE_SECURE_CONFIG_PASSPHRASE.\nKey length shall be 16 (AES-128), 24 (AES-192), or 32 (AES-256) bytes in length.")

	mountCmd.PersistentFlags().String("log-type", "syslog", "Type of logger to be used by the system. Set to syslog by default. Allowed values are silent|syslog|base.")
	config.BindPFlag("logging.type", mountCmd.PersistentFlags().Lookup("log-type"))
	_ = mountCmd.RegisterFlagCompletionFunc("log-type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"silent", "base", "syslog"}, cobra.ShellCompDirectiveNoFileComp
	})

	mountCmd.PersistentFlags().String("log-level", "LOG_WARNING",
		"Enables logs written to syslog. Set to LOG_WARNING by default. Allowed values are LOG_OFF|LOG_CRIT|LOG_ERR|LOG_WARNING|LOG_INFO|LOG_DEBUG")
	config.BindPFlag("logging.level", mountCmd.PersistentFlags().Lookup("log-level"))
	_ = mountCmd.RegisterFlagCompletionFunc("log-level", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"LOG_OFF", "LOG_CRIT", "LOG_ERR", "LOG_WARNING", "LOG_INFO", "LOG_TRACE", "LOG_DEBUG"}, cobra.ShellCompDirectiveNoFileComp
	})

	mountCmd.PersistentFlags().String("log-file-path",
		common.DefaultLogFilePath, "Configures the path for log files. Default is "+common.DefaultLogFilePath)
	config.BindPFlag("logging.file-path", mountCmd.PersistentFlags().Lookup("log-file-path"))
	_ = mountCmd.MarkPersistentFlagDirname("log-file-path")

	mountCmd.PersistentFlags().Bool("foreground", false, "Mount the system in foreground mode. Default value false. (Ignored on Windows)")
	config.BindPFlag("foreground", mountCmd.PersistentFlags().Lookup("foreground"))

	mountCmd.PersistentFlags().Bool("read-only", false, "Mount the system in read only mode. Default value false.")
	config.BindPFlag("read-only", mountCmd.PersistentFlags().Lookup("read-only"))

	mountCmd.PersistentFlags().String("default-working-dir", "", "Default working directory for storing log files and other lyvecloudfuse information")
	mountCmd.PersistentFlags().Lookup("default-working-dir").Hidden = true
	config.BindPFlag("default-working-dir", mountCmd.PersistentFlags().Lookup("default-working-dir"))
	_ = mountCmd.MarkPersistentFlagDirname("default-working-dir")

	mountCmd.Flags().BoolVar(&options.Streaming, "streaming", false, "Enable Streaming.")
	config.BindPFlag("streaming", mountCmd.Flags().Lookup("streaming"))
	mountCmd.Flags().Lookup("streaming").Hidden = true

	mountCmd.Flags().BoolVar(&options.AttrCache, "use-attr-cache", true, "Use attribute caching.")
	config.BindPFlag("use-attr-cache", mountCmd.Flags().Lookup("use-attr-cache"))
	mountCmd.Flags().Lookup("use-attr-cache").Hidden = true

	mountCmd.Flags().Bool("invalidate-on-sync", true, "Invalidate file/dir on sync/fsync.")
	config.BindPFlag("invalidate-on-sync", mountCmd.Flags().Lookup("invalidate-on-sync"))
	mountCmd.Flags().Lookup("invalidate-on-sync").Hidden = true

	mountCmd.Flags().Bool("pre-mount-validate", true, "Validate lyvecloudfuse is mounted.")
	config.BindPFlag("pre-mount-validate", mountCmd.Flags().Lookup("pre-mount-validate"))
	mountCmd.Flags().Lookup("pre-mount-validate").Hidden = true

	mountCmd.Flags().Bool("basic-remount-check", true, "Validate lyvecloudfuse is mounted by reading /etc/mtab.")
	config.BindPFlag("basic-remount-check", mountCmd.Flags().Lookup("basic-remount-check"))
	mountCmd.Flags().Lookup("basic-remount-check").Hidden = true

	mountCmd.PersistentFlags().StringSliceVarP(&options.LibfuseOptions, "o", "o", []string{}, "FUSE options.")
	config.BindPFlag("libfuse-options", mountCmd.PersistentFlags().ShorthandLookup("o"))
	mountCmd.PersistentFlags().ShorthandLookup("o").Hidden = true

	mountCmd.PersistentFlags().DurationVar(&options.WaitForMount, "wait-for-mount", 5*time.Second, "Let parent process wait for given timeout before exit")

	config.AttachToFlagSet(mountCmd.PersistentFlags())
	config.AttachFlagCompletions(mountCmd)
	config.AddConfigChangeEventListener(config.ConfigChangeEventHandlerFunc(OnConfigChange))
}

func Destroy(message string) error {
	_ = log.Destroy()
	return fmt.Errorf(message)
}
