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
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
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

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"
	"github.com/awnumar/memguard"

	"github.com/sevlyar/go-daemon"
	"github.com/spf13/cobra"
)

type LogOptions struct {
	Type           string `config:"type"             yaml:"type,omitempty"`
	LogLevel       string `config:"level"            yaml:"level,omitempty"`
	LogFilePath    string `config:"file-path"        yaml:"file-path,omitempty"`
	MaxLogFileSize uint64 `config:"max-file-size-mb" yaml:"max-file-size-mb,omitempty"`
	LogFileCount   uint64 `config:"file-count"       yaml:"file-count,omitempty"`
	TimeTracker    bool   `config:"track-time"       yaml:"track-time,omitempty"`
}

type mountOptions struct {
	MountPath  string
	ConfigFile string

	DryRun              bool
	Logging             LogOptions     `config:"logging"`
	Components          []string       `config:"components"`
	Foreground          bool           `config:"foreground"`
	NonEmpty            bool           `config:"nonempty"`
	DefaultWorkingDir   string         `config:"default-working-dir"`
	CPUProfile          string         `config:"cpu-profile"`
	MemProfile          string         `config:"mem-profile"`
	PassPhrase          string         `config:"passphrase"`
	SecureConfig        bool           `config:"secure-config"`
	DynamicProfiler     bool           `config:"dynamic-profile"`
	ProfilerPort        int            `config:"profiler-port"`
	ProfilerIP          string         `config:"profiler-ip"`
	MonitorOpt          monitorOptions `config:"health_monitor"`
	WaitForMount        time.Duration  `config:"wait-for-mount"`
	LazyWrite           bool           `config:"lazy-write"`
	EnableRemountUser   bool
	EnableRemountSystem bool
	PassphrasePipe      string

	// v1 support
	Streaming         bool     `config:"streaming"`
	AttrCache         bool     `config:"use-attr-cache"`
	LibfuseOptions    []string `config:"libfuse-options"`
	BlockCache        bool     `config:"block-cache"`
	EntryCacheTimeout int      `config:"list-cache-timeout"`
}

var options mountOptions

func (opt *mountOptions) validate(skipNonEmptyMount bool) error {
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
		} else if !skipNonEmptyMount && !common.IsDirectoryEmpty(opt.MountPath) {
			return fmt.Errorf("mount directory is not empty")
		}

		if common.IsDirectoryMounted(opt.MountPath) {
			// Try to cleanup the stale mount
			log.Info("Mount::validate : Mount directory is already mounted, trying to cleanup")
			active, err := common.IsMountActive(opt.MountPath)
			if active || err != nil {
				// Previous mount is still active so we need to fail this mount
				return fmt.Errorf("directory is already mounted")
			} else {
				// Previous mount is in stale state so lets cleanup the state
				log.Info("Mount::validate : Cleaning up stale mount")
				if err = unmountCloudfuse(opt.MountPath, true); err != nil {
					return fmt.Errorf("directory is already mounted, unmount manually before remount [%v]", err.Error())
				}

				// Clean up the file-cache temp directory if any
				var tempCachePath string
				_ = config.UnmarshalKey("file_cache.path", &tempCachePath)

				var cleanupOnStart bool
				_ = config.UnmarshalKey("file_cache.cleanup-on-start", &cleanupOnStart)

				if tempCachePath != "" && cleanupOnStart {
					if err = common.TempCacheCleanup(tempCachePath); err != nil {
						return fmt.Errorf("failed to cleanup file cache [%s]", err.Error())
					}
				}
			}
		} else if !skipNonEmptyMount && !common.IsDirectoryEmpty(opt.MountPath) {
			return fmt.Errorf("mount directory is not empty")
		}
	}

	if err := common.ELogLevel.Parse(opt.Logging.LogLevel); err != nil {
		return fmt.Errorf("invalid log level [%s]", err.Error())
	}

	if opt.DefaultWorkingDir != "" {
		common.DefaultWorkDir = opt.DefaultWorkingDir

		if opt.Logging.LogFilePath == common.DefaultLogFilePath {
			// If default-working-dir is set then default log path shall be set to that path
			// Ignore if specific log-path is provided by user
			opt.Logging.LogFilePath = common.JoinUnixFilepath(
				common.DefaultWorkDir,
				"cloudfuse.log",
			)
		}

		common.DefaultLogFilePath = common.JoinUnixFilepath(common.DefaultWorkDir, "cloudfuse.log")
	}

	err := common.CreateDefaultDirectory()
	if err != nil {
		return fmt.Errorf("failed to create default work dir [%s]", err.Error())
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
		if options.PassPhrase == "" && options.PassphrasePipe == "" {
			options.PassPhrase = os.Getenv(SecureConfigEnvName)
			if options.PassPhrase == "" {
				return errors.New(
					"no passphrase provided to decrypt the config file.\n Either use --passphrase cli option or store passphrase in CLOUDFUSE_SECURE_CONFIG_PASSPHRASE environment variable",
				)
			}

			_, err := base64.StdEncoding.DecodeString(string(options.PassPhrase))
			if err != nil {
				return fmt.Errorf("passphrase is not valid base64 encoded [%s]", err.Error())
			}
		} else if options.PassphrasePipe != "" && runtime.GOOS == "windows" {
			var err error
			options.PassPhrase, err = readPassphraseFromPipe(options.PassphrasePipe, 10*time.Second)
			if err != nil {
				return errors.New(
					"no passphrase read from named pipe",
				)
			}
		}

		encryptedPassphrase = memguard.NewEnclave([]byte(options.PassPhrase))

		cipherText, err := os.ReadFile(options.ConfigFile)
		if err != nil {
			return fmt.Errorf(
				"failed to read encrypted config file %s [%s]",
				options.ConfigFile,
				err.Error(),
			)
		}

		plainText, err := common.DecryptData(cipherText, encryptedPassphrase)
		if err != nil {
			return fmt.Errorf(
				"failed to decrypt config file %s [%s]",
				options.ConfigFile,
				err.Error(),
			)
		}

		config.SetConfigFile(options.ConfigFile)
		config.SetSecureConfigOptions(encryptedPassphrase)
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

// We use the cobra library to provide a CLI for Cloudfuse.
// Look at https://cobra.dev/ for more information
var mountCmd = &cobra.Command{
	Use:               "mount <mount path>",
	Short:             "Mount the container as a filesystem",
	Long:              "Mount the container as a filesystem",
	SuggestFor:        []string{"mnt", "mout"},
	Args:              cobra.ExactArgs(1),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(_ *cobra.Command, args []string) error {
		options.MountPath = common.ExpandPath(args[0])
		common.MountPath = options.MountPath

		if options.ConfigFile == "" {
			// Config file is not set in cli parameters
			// Cloudfuse defaults to config.yaml in current directory
			// If the file does not exist then user might have configured required things in env variables
			// Fall back to defaults and let components fail if all required env variables are not set.
			_, err := os.Stat(common.DefaultConfigFilePath)
			if err != nil && os.IsNotExist(err) {
				return errors.New("failed to initialize new pipeline :: config file not provided")
			} else {
				options.ConfigFile = common.DefaultConfigFilePath
			}
		}

		err := parseConfig()
		if err != nil {
			return err
		}

		err = config.Unmarshal(&options)
		if err != nil {
			return fmt.Errorf("failed to unmarshal config [%s]", err.Error())
		}

		options.Foreground = options.Foreground || options.DryRun

		// handle Windows background mount (formerly "service mount")
		if !options.Foreground && runtime.GOOS == "windows" {
			// validate mount path
			options.MountPath = strings.ReplaceAll(common.ExpandPath(args[0]), "\\", "/")
			if options.MountPath == "" {
				return errors.New("mount path not provided")
			}
			if _, err := os.Stat(options.MountPath); errors.Is(err, fs.ErrExist) || err == nil {
				return errors.New("mount path exists")
			}
			// Convert the path into a full path so WinFSP can see the config file
			configPath, err := filepath.Abs(options.ConfigFile)
			if err != nil {
				return errors.New("config file does not exist")
			}
			options.ConfigFile = configPath
			if _, err := os.Stat(options.ConfigFile); errors.Is(err, fs.ErrNotExist) {
				return errors.New("config file does not exist")
			}
			// mount using WinFSP, and persist on reboot
			err = createMountInstance(options.EnableRemountUser, options.EnableRemountSystem)
			if err != nil {
				return fmt.Errorf("failed to mount instance [%s]", err.Error())
			}

			return nil
		}

		if len(options.Components) == 0 {
			pipeline := []string{"libfuse"}

			if config.IsSet("streaming") && options.Streaming {
				pipeline = append(pipeline, "stream")
			} else if config.IsSet("block-cache") && options.BlockCache {
				pipeline = append(pipeline, "block_cache")
			} else {
				pipeline = append(pipeline, "file_cache")
			}

			// by default attr-cache is enable in v2
			// only way to disable is to pass cli param and set it to false
			if options.AttrCache {
				pipeline = append(pipeline, "attr_cache")
			}

			if containers, err := getBucketListS3(); len(containers) != 0 && err == nil {
				pipeline = append(pipeline, "s3storage")
			} else if containers, err = getContainerListAzure(); len(containers) != 0 && err == nil {
				pipeline = append(pipeline, "azstorage")
			} else {
				return errors.New("failed to initialize new pipeline :: unable to determine cloud provider. no pipeline components found in the config: " + err.Error())
			}
			options.Components = pipeline
		}

		if config.IsSet("entry_cache.timeout-sec") || options.EntryCacheTimeout > 0 {
			options.Components = append(
				options.Components[:1],
				append([]string{"entry_cache"}, options.Components[1:]...)...)
		}

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
					config.Set("lfuse.attribute-expiration-sec", parameter[1])
				} else if strings.HasPrefix(v, "entry_timeout=") {
					config.Set("lfuse.entry-expiration-sec", parameter[1])
				} else if strings.HasPrefix(v, "negative_timeout=") {
					config.Set("lfuse.negative-entry-expiration-sec", parameter[1])
				} else if v == "ro" || v == "ro=true" {
					config.Set("read-only", "true")
				} else if v == "allow_root" || v == "allow_root=true" {
					config.Set("allow-root", "true")
				} else if v == "nonempty" || v == "nonempty=true" {
					// For fuse3, -o nonempty mount option has been removed and
					// mounting over non-empty directories is now always allowed.
					// For fuse2, this option is supported.
					options.NonEmpty = true
					config.Set("nonempty", "true")
				} else if strings.HasPrefix(v, "umask=") {
					umask, err := strconv.ParseUint(parameter[1], 10, 32)
					if err != nil {
						return fmt.Errorf("failed to parse umask [%s]", err.Error())
					}
					config.Set("lfuse.umask", fmt.Sprint(umask))
				} else if strings.HasPrefix(v, "uid=") {
					val, err := strconv.ParseUint(parameter[1], 10, 32)
					if err != nil {
						return fmt.Errorf("failed to parse uid [%s]", err.Error())
					}
					config.Set("lfuse.uid", fmt.Sprint(val))
				} else if strings.HasPrefix(v, "gid=") {
					val, err := strconv.ParseUint(parameter[1], 10, 32)
					if err != nil {
						return fmt.Errorf("failed to parse gid [%s]", err.Error())
					}
					config.Set("lfuse.gid", fmt.Sprint(val))
				} else if v == "direct_io" || v == "direct_io=true" {
					config.Set("lfuse.direct-io", "true")
					config.Set("direct-io", "true")
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

		err = options.validate(options.NonEmpty)
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

		// TODO: remove v1 switches, which were never used as part of cloudfuse.
		if config.IsSet("invalidate-on-sync") {
			log.Warn(
				"mount: unsupported v1 CLI parameter: invalidate-on-sync is always true in cloudfuse.",
			)
		}
		if config.IsSet("pre-mount-validate") {
			log.Warn(
				"mount: unsupported v1 CLI parameter: pre-mount-validate is always true in cloudfuse.",
			)
		}
		if config.IsSet("basic-remount-check") {
			log.Warn(
				"mount: unsupported v1 CLI parameter: basic-remount-check is always true in cloudfuse.",
			)
		}

		common.EnableMonitoring = options.MonitorOpt.EnableMon

		// check if cloudfuse stats monitor is added in the disable list
		for _, mon := range options.MonitorOpt.DisableList {
			if mon == common.CfuseStats {
				common.CfsDisabled = true
				break
			}
		}

		config.Set("mount-path", options.MountPath)

		var pipeline *internal.Pipeline

		log.Crit(
			"Starting Cloudfuse Mount : %s on [%s]",
			common.CloudfuseVersion,
			common.GetCurrentDistro(),
		)
		log.Info("Mount Command: %s", os.Args)
		log.Crit("Logging level set to : %s", logLevel.String())
		log.Debug("Mount allowed on nonempty path : %v", options.NonEmpty)

		directIO := false
		_ = config.UnmarshalKey("direct-io", &directIO)
		if directIO {
			// Directio is enabled, so remove the attr-cache from the pipeline
			for i, name := range options.Components {
				if name == "attr_cache" {
					options.Components = append(options.Components[:i], options.Components[i+1:]...)
					log.Crit(
						"Mount::runPipeline : Direct IO enabled, removing attr_cache from pipeline",
					)
					break
				}
			}
		}

		// If on Linux start with the go daemon
		// If on Windows, don't use the daemon since it is not supported
		if runtime.GOOS == "windows" {
			pipeline, err = internal.NewPipeline(options.Components, true)
		} else {
			pipeline, err = internal.NewPipeline(options.Components, !daemon.WasReborn())
		}

		if err != nil {
			if err.Error() == "Azure CLI not found on path" {
				log.Err(
					"mount : failed to initialize new pipeline :: To authenticate using MSI with object-ID, ensure Azure CLI is installed. Alternatively, use app/client ID or resource ID for authentication. [%v]",
					err,
				)
				return Destroy(
					fmt.Sprintf(
						"failed to initialize new pipeline :: To authenticate using MSI with object-ID, ensure Azure CLI is installed. Alternatively, use app/client ID or resource ID for authentication. [%s]",
						err.Error(),
					),
				)
			}

			log.Err("mount :  failed to initialize new pipeline [%v]", err)
			return Destroy(
				fmt.Sprintf("mount : failed to initialize new pipeline [%s]", err.Error()),
			)
		}

		// Dry run ends here
		if options.DryRun {
			log.Trace("Dry-run complete")
			return nil
		}

		common.ForegroundMount = options.Foreground

		log.Info("mount: Mounting cloudfuse on %s", options.MountPath)
		// handle background mount on Linux
		if !options.Foreground && runtime.GOOS != "windows" {
			pidFile := strings.ReplaceAll(options.MountPath, "/", "_") + ".pid"
			pidFileName := filepath.Join(os.ExpandEnv(common.DefaultWorkDir), pidFile)

			// Delete the pidFile if it already exists which prevents a failed to daemonize error
			// See https://github.com/sevlyar/go-daemon/issues/37
			err := os.Remove(pidFileName)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("mount: failed to remove pidFile [%v]", err.Error())
			}

			pid := os.Getpid()
			fname := fmt.Sprintf("/tmp/cloudfuse.%v", pid)

			ctx := context.Background()
			err = createDaemon(pipeline, ctx, pidFileName, 0644, 022, fname)
			if err != nil {
				return fmt.Errorf("mount: failed to create daemon [%v]", err.Error())
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
	log.Debug(
		"Mount::runPipeline : cloudfuse pid = %v, transfer pipe = %v, polling pipe = %v",
		pid,
		common.TransferPipe,
		common.PollingPipe,
	)

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
		rootCmd.SetArgs(
			[]string{
				"health-monitor",
				fmt.Sprintf("--pid=%v", pid),
				fmt.Sprintf("--config-file=%s", options.ConfigFile),
			},
		)
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
		"Base64 encoded key to decrypt config file. Can also be specified by env-variable CLOUDFUSE_SECURE_CONFIG_PASSPHRASE.\n Decoded key length shall be 16 (AES-128), 24 (AES-192), or 32 (AES-256) bytes in length.")

	mountCmd.PersistentFlags().
		String("log-type", "base", "Type of logger to be used by the system. Set to base by default. Allowed values are silent|syslog|base.")
	config.BindPFlag("logging.type", mountCmd.PersistentFlags().Lookup("log-type"))
	_ = mountCmd.RegisterFlagCompletionFunc(
		"log-type",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"silent", "base", "syslog"}, cobra.ShellCompDirectiveNoFileComp
		},
	)

	mountCmd.PersistentFlags().String("log-level", "LOG_WARNING",
		"Enables logs written to syslog. Set to LOG_WARNING by default. Allowed values are LOG_OFF|LOG_CRIT|LOG_ERR|LOG_WARNING|LOG_INFO|LOG_DEBUG")
	config.BindPFlag("logging.level", mountCmd.PersistentFlags().Lookup("log-level"))
	_ = mountCmd.RegisterFlagCompletionFunc(
		"log-level",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{
				"LOG_OFF",
				"LOG_CRIT",
				"LOG_ERR",
				"LOG_WARNING",
				"LOG_INFO",
				"LOG_TRACE",
				"LOG_DEBUG",
			}, cobra.ShellCompDirectiveNoFileComp
		},
	)

	mountCmd.PersistentFlags().String("log-file-path",
		common.DefaultLogFilePath, "Configures the path for log files. Default is "+common.DefaultLogFilePath)
	config.BindPFlag("logging.file-path", mountCmd.PersistentFlags().Lookup("log-file-path"))
	_ = mountCmd.MarkPersistentFlagDirname("log-file-path")

	mountCmd.PersistentFlags().
		Bool("foreground", false, "Mount the system in foreground mode. Default value false.")
	config.BindPFlag("foreground", mountCmd.PersistentFlags().Lookup("foreground"))

	mountCmd.PersistentFlags().
		Bool("read-only", false, "Mount the system in read only mode. Default value false.")
	config.BindPFlag("read-only", mountCmd.PersistentFlags().Lookup("read-only"))

	mountCmd.Flags().BoolVar(&options.DryRun, "dry-run", false,
		"Test mount configuration, credentials, etc., but don't make any changes to the container or the local file system. Implies foreground.")
	config.BindPFlag("dry-run", mountCmd.Flags().Lookup("dry-run"))

	mountCmd.PersistentFlags().
		Bool("lazy-write", false, "Async write to storage container after file handle is closed.")
	config.BindPFlag("lazy-write", mountCmd.PersistentFlags().Lookup("lazy-write"))

	mountCmd.PersistentFlags().
		String("default-working-dir", "", "Default working directory for storing log files and other cloudfuse information")
	mountCmd.PersistentFlags().Lookup("default-working-dir").Hidden = true
	config.BindPFlag(
		"default-working-dir",
		mountCmd.PersistentFlags().Lookup("default-working-dir"),
	)
	_ = mountCmd.MarkPersistentFlagDirname("default-working-dir")

	mountCmd.Flags().BoolVar(&options.Streaming, "streaming", false, "Enable Streaming.")
	config.BindPFlag("streaming", mountCmd.Flags().Lookup("streaming"))
	mountCmd.Flags().Lookup("streaming").Hidden = true

	mountCmd.Flags().BoolVar(&options.BlockCache, "block-cache", false, "Enable Block-Cache.")
	config.BindPFlag("block-cache", mountCmd.Flags().Lookup("block-cache"))
	mountCmd.Flags().Lookup("block-cache").Hidden = true

	mountCmd.Flags().BoolVar(&options.AttrCache, "use-attr-cache", true, "Use attribute caching.")
	config.BindPFlag("use-attr-cache", mountCmd.Flags().Lookup("use-attr-cache"))
	mountCmd.Flags().Lookup("use-attr-cache").Hidden = true

	mountCmd.Flags().Bool("invalidate-on-sync", true, "Invalidate file/dir on sync/fsync.")
	config.BindPFlag("invalidate-on-sync", mountCmd.Flags().Lookup("invalidate-on-sync"))
	mountCmd.Flags().Lookup("invalidate-on-sync").Hidden = true

	mountCmd.Flags().Bool("pre-mount-validate", true, "Validate cloudfuse is mounted.")
	config.BindPFlag("pre-mount-validate", mountCmd.Flags().Lookup("pre-mount-validate"))
	mountCmd.Flags().Lookup("pre-mount-validate").Hidden = true

	mountCmd.Flags().
		Bool("basic-remount-check", true, "Validate cloudfuse is mounted by reading /etc/mtab.")
	config.BindPFlag("basic-remount-check", mountCmd.Flags().Lookup("basic-remount-check"))
	mountCmd.Flags().Lookup("basic-remount-check").Hidden = true

	if runtime.GOOS == "windows" {
		mountCmd.Flags().
			BoolVar(&options.EnableRemountSystem, "enable-remount-system", false, "Remount container on server restart. Mount will restart on reboot.")
		config.BindPFlag("enable-remount-system", mountCmd.Flags().Lookup("enable-remount-system"))

		mountCmd.Flags().
			BoolVar(&options.EnableRemountUser, "enable-remount-user", false, "Remount container on server restart for current user. Mount will restart on current user log in.")
		config.BindPFlag("enable-remount-user", mountCmd.Flags().Lookup("enable-remount-user"))

		mountCmd.Flags().
			StringVar(&options.PassphrasePipe, "passphrase-pipe", "", "Specifies a named pipe to read the passphrase from.")
		config.BindPFlag("passphrase-pipe", mountCmd.Flags().Lookup("passphrase-pipe"))
	}

	mountCmd.PersistentFlags().
		StringSliceVarP(&options.LibfuseOptions, "o", "o", []string{}, "FUSE options.")
	config.BindPFlag("libfuse-options", mountCmd.PersistentFlags().ShorthandLookup("o"))
	mountCmd.PersistentFlags().ShorthandLookup("o").Hidden = true

	mountCmd.PersistentFlags().
		DurationVar(&options.WaitForMount, "wait-for-mount", 5*time.Second, "Let parent process wait for given timeout before exit")

	config.AttachToFlagSet(mountCmd.PersistentFlags())
	config.AttachFlagCompletions(mountCmd)
	config.AddConfigChangeEventListener(config.ConfigChangeEventHandlerFunc(OnConfigChange))
}

func Destroy(message string) error {
	_ = log.Destroy()
	if message != "" {
		return fmt.Errorf("%s", message)
	}

	return nil
}
