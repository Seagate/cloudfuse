# /lyvecloudfuse/setup/baseConfig.yaml
# comments duplicated from baseconfig

default_config_common = {
    'foreground': False,    # run lyvecloudfuse in foreground or background
    'allow-other': True,    # mount in read only mode - used for Streaming and FUSE
    'read-only' : False,    # allow other users to access the mounted directory - used for FUSE and File Cache
    'nonempty' : False      # allow mounting on non-empty directory - used for FUSE
}


# All the options for the pipeline set-up. Not including debug
azure_pipeline_filecache = {
    'components' : [
        'libfuse',
        'file_cache',
        'attr_cache',
        'azstorage'
        ]
}
azure_pipeline_streaming = {
    'components' : [
        'libfuse',
        'stream',
        'attr_cache',
        'azstorage'
    ]
}
lyve_pipeline_filecache = {
    'components' : [
        'libfuse',
        'file_cache',
        'attr_cache',
        's3storage'
        ]   
}
lyve_pipeline_streaming = {
    'components' : [
        'libfuse',
        'stream',
        'attr_cache',
        's3storage'
    ]
}


# Libfuse configurations
default_libfuse = {
    'libfuse' : {
        'default-permission' : 0o777,               # 0o777|0o666|0o644|0o444 default permissions to be presented for block blobs
        'attribute-expiration-sec': 120,            # time kernel can cache inode attributes (in sec)
        'entry-expiration-sec' : 120,               # time kernel can cache directory listing attributes (in sec)
        'negative-entry-expiration-sec' : 120,      # time kernel can cache attributes of non existent paths (in sec)
        'fuse-trace' : False,                       # enable libfuse api trace logs for debugging
        'extension' : '',                           # physical path to extension library
        
        'disable-writeback-cache' : False,          # disallow libfuse to buffer write requests if you must strictly open files in O_WRONLY or O_APPEND mode. 
                                                    # alternatively, you can ignore-open-flags
        'ignore-open-flags' : True                  # ignore the append and write only flag since O_APPEND and O_WRONLY is not supported with writeback caching. alternatively, you can disable-writeback-cache
    }
}

# Dynamic profiler related configuration. This helps to root-cause high memory/cpu usage related issues.
default_dynamicProfiler = {
    'dynamic-profile' : False,   # allows to turn on dynamic profiler for cpu/memory usage monitoring. Only for debugging, shall not be used in production
    'profiler-port' : 6060,      # port number for dynamic-profiler to listen for REST calls
    'profiler-ip' : 'localhost'    # IP address for dynamic-profiler to listen for REST calls
}

# Logger configs, for future use
default_logging = {
    'logging' : {
        'type' : 'syslog',                                              # syslog|silent|base
        'level' : 'log_warning',                                        # log_off|log_crit|log_err|log_warning|log_info|log_trace|log_debug
        'file-path' : '$HOME/.lyvecloudfuse/lyvecloudfuse.log',         # path where log files shall be stored
        'max-file-size-mb' : 512,                                       # maximum allowed size for each log file (in MB)
        'file-count' : 10 ,                                             # maximum number of files to be rotated to preserve old logs
        'track-time' : False                                            # track time taken by important operations
    }
}


# Streaming configuration
default_streaming = {
    'stream' : {
        # If block-size-mb, blocks-per-file or cache-size-mb are 0, the stream component will not cache blocks. 
        'block-size-mb' : 0,        # for read only mode:: size of each block to be cached in memory while streaming (in MB). For read/write:: size of newly created blocks
        'max-buffers' : 0,          # total number of buffers to store blocks in
        'buffer-size-mb' : 0,       # size for each buffer
        'file-caching' : False      # read/write mode file level caching or handle level caching. Default - false (handle level caching ON)
    }
}

# Disk cache related configuration
default_filecache = {
    'file_cache' : {
        # Required
        'path' : '',                     # path to local disk cache
        # Optional
        'policy' : 'lru',                # lru|lfu eviction policy to be engaged for cache eviction. lru = least recently used file to be deleted, lfu = least frequently used file to be deleted
        'timeout-sec' : 120,             # default cache eviction timeout (in sec)
        'max-eviction' : 5000,           # number of files that can be evicted at once
        'max-size-mb' : 0,               # maximum cache size allowed. 0 = unlimited
        'high-threshold' : 80,           # % disk space consumed which triggers eviction. This parameter overrides 'timeout-sec' parameter and cached files will be removed even if they have not expired
        'low-threshold' : 60,            # % disk space consumed which triggers eviction to stop when previously triggered by the high-threshold
        'create-empty-file' : False,     # create an empty file on container when create call is received from kernel
        'allow-non-empty-temp' : False,  # allow non empty temp directory at startup
        'cleanup-on-start' : False,      # cleanup the temp directory on startup, if its not empty
        'policy-trace' : False,          # generate eviction policy logs showing which files will expire soon
        'offload-io' : False             # by default libfuse will service reads/writes to files for better perf. Set to true to make file-cache component service read/write calls
        }
}

# Attribute cache related configuration
default_attribute_cache = {
    'timeout-sec' : 120,         # time attributes can be cached (in sec)
    'no-cache-on-list' : False,  # do not cache attributes during listing, to optimize performance
    'no-symlinks' : False        # to improve performance disable symlink support. symlinks will be treated like regular files
}

# Loopback configuration
default_loopbackfs = {
    'path' : ''     # path to local directory
}

# Azure storage configuration
default_azstorage = {
    'azstorage' : {
        'type' : 'block',               # block|adls type of storage account to be connected
        'account-name' : '',            # name of the storage account
        'container' : '',               # name of the storage container to be mounted
        'endpoint' : '',                # storage account endpoint (example - https://account-name.blob.core.windows.net)
        'mode' : '',                    # key|sas|spn|msi kind of authentication to be used
      
        # The following are the options depending on mode selected
        'account-key': '',              # storage account key
        # OR
        'sas': '',                      # storage account sas
        # OR
        'appid' : '',                   # storage account app id / client id for MSI
        'resid' : '',                   # storage account resource id for MSI
        'objid' : '',                   # object id for MSI
        #OR
        'tenantid' : '',                # storage account tenant id for SPN
        'clientid' : '',                # storage account client id for SPN
        'clientsecret' : '',            # storage account client secret for SPN
        # Optional
        'use-http': False,              # use http instead of https for storage connection
        'aadendpoint' : '',             # storage account custom aad endpoint
        'subdirectory' : '',            # name of subdirectory to be mounted instead of whole container
        'block-size-mb' : 16,           # size of each block (in MB)
        'max-concurrency' : 32,         # number of parallel upload/download threads
        'tier' : 'none',                # hot|cool|archive|none <blob-tier to be set while uploading a blob
        'block-list-on-mount-sec' : 0,  # time list api to be blocked after mount (in sec)
        'max-retries' : 5,              # number of retries to attempt for any operation failure
        'max-retry-timeout-sec' : 900,  # maximum timeout allowed for a given retry (in sec)
        'retry-backoff-sec' : 4,        # retry backoff between two tries (in sec)
        'max-retry-delay-sec' : 60,     # maximum delay between two tries (in sec)
        'http-proxy' : '',              # ip-address:port <http proxy to be used for connection
        'https-proxy' : '',             # ip-address:port <https proxy to be used for connection
        'sdk-trace' : False,            # enable storage sdk logging
        'fail-unsupported-op' : False,  # for block blob account return failure for unsupported operations like chmod and chown
        'auth-resource' : '',           # resource string to be used during OAuth token retrieval
        'update-md5': False,            # set md5 sum on upload. Impacts performance. works only when file-cache component is part of the pipeline
        'validate-md5' : False,         # validate md5 on download. Impacts performance. works only when file-cache component is part of the pipeline
        'virtual-directory' : False     # support virtual directories without existence of a special marker blob
        }
}


# Mount all configuration
default_mountall = {
    'mountall' : {
        # allowlist takes precedence over denylist in case of conflicts
        'container-allowlist' : [], # list of containers to be mounted
        'container-denylist' : [] # list of containers not to be mounted
    }
}


# Health Monitor configuration
default_health_monitor = {
    'health_monitor' : {
        'enable-monitoring' : False,            # enable health monitor
        'stats-poll-interval-sec' : 10,         # Lyvecloudfuse stats polling interval (in sec)
        'process-monitor-interval-sec' : 30,    # CPU, memory and network usage polling interval (in sec)
        'output-path' : ''                      # Path where health monitor will generate its output file. File name will be monitor_<pid>.json
    }
}

# list of monitors to be disabled
default_monitor_disable = {
    'monitor-disable-list': [
        'blobfuse_stats',       # Disable lyvecloudfuse stats polling
        'file_cache_monitor',   # Disable file cache directory monitor
        'cpu_profiler',         # Disable CPU monitoring on lyvecloudfuse process
        'memory_profiler',      # Disable memory monitoring on lyvecloudfuse process
        'network_profiler'      # Disable network monitoring on lyvecloudfuse process
    ]
}