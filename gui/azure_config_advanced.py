"""
This module defines the AzureAdvancedSettingsWidget class for configuring
advanced Azure settings.
"""

# Licensed under the MIT License <http://opensource.org/licenses/MIT>.
#
# Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE

from sys import platform
from PySide6.QtCore import QSettings

# import the custom class made from QtDesigner
from utils import set_path_validator, populate_widgets_from_settings, update_settings_from_widgets
from ui_azure_config_advanced import Ui_Form
from common_qt_functions import WidgetCustomFunctions

file_cache_eviction_choices = ['lru', 'lfu']
az_blob_tier = ['none', 'hot', 'cool', 'archive']


class AzureAdvancedSettingsWidget(WidgetCustomFunctions, Ui_Form):
    """
    A widget for configuring advanced Azure settings.

    Attributes:
        settings (dict): Configuration settings for Azure.
        my_window (QSettings): QSettings object for storing window state.
        saveButtonClicked (bool): Flag to indicate if the save button was clicked.
    """
    def __init__(self, config_settings: dict):
        """
        Initialize the AzureAdvancedSettingsWidget with the given configuration settings.

        Args:
            config_settings (dict): Configuration settings for Azure.
        """
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle('Advanced Azure Config Settings')
        self.settings = config_settings
        self.my_window = QSettings('Cloudfuse', 'AzAdvancedWindow')
        
        self._az_storage_mapping = {
            'use-http': self.checkBox_azure_useHttp,
            'validate-md5': self.checkBox_azure_validateMd5,
            'update-md5': self.checkBox_azure_updateMd5,
            'fail-unsupported-op': self.checkBox_azure_failUnsupportedOps,
            'sdk-trace': self.checkBox_azure_sdkTrace,
            'virtual-directory': self.checkBox_azure_virtualDirectory,
            'disable-compression': self.checkBox_azure_disableCompression,
            'block-size-mb': self.spinBox_azure_blockSize,
            'max-concurrency': self.spinBox_azure_maxConcurrency,
            'block-list-on-mount-sec': self.spinBox_azure_blockOnMount,
            'max-retries': self.spinBox_azure_maxRetries,
            'max-retry-timeout-sec': self.spinBox_azure_maxRetryTimeout,
            'retry-backoff-sec': self.spinBox_azure_retryBackoff,
            'max-retry-delay-sec': self.spinBox_azure_maxRetryDelay,
            'aadendpoint': self.lineEdit_azure_aadEndpoint,
            'subdirectory': self.lineEdit_azure_subDirectory,
            'http-proxy': self.lineEdit_azure_httpProxy,
            'https-proxy': self.lineEdit_azure_httpsProxy,
            'auth-resource': self.lineEdit_azure_authResource,
            'tier': self.dropDown_azure_blobTier,
        }
        self._libfuse_mapping = {
            'disable-writeback-cache': self.checkBox_libfuse_disableWriteback,
            'network-share': self.checkBox_libfuse_networkshare,
            'max-fuse-threads': self.spinBox_libfuse_maxFuseThreads,
        }
        self._file_cache_mapping = {
            'allow-non-empty-temp': self.checkBox_fileCache_allowNonEmptyTmp,
            'policy-trace': self.checkBox_fileCache_policyLogs,
            'create-empty-file': self.checkBox_fileCache_createEmptyFile,
            'cleanup-on-start': self.checkBox_fileCache_cleanupStart,
            'offload-io': self.checkBox_fileCache_offloadIO,
            'sync-to-flush': self.checkBox_fileCache_syncToFlush,
            'timeout-sec': self.spinBox_fileCache_evictionTimeout,
            'max-eviction': self.spinBox_fileCache_maxEviction,
            'max-size-mb': self.spinBox_fileCache_maxCacheSize,
            'high-threshold': self.spinBox_fileCache_evictMaxThresh,
            'low-threshold': self.spinBox_fileCache_evictMinThresh,
            'refresh-sec': self.spinBox_fileCache_refreshSec,
            'policy': self.dropDown_fileCache_evictionPolicy,
        }
        
        self.init_window_size_pos()
        self.populate_options()
        self._save_button_clicked = False

        set_path_validator(self.lineEdit_azure_subDirectory)

        # Set up the signals
        self.button_okay.clicked.connect(self.exit_window)
        self.button_resetDefaultSettings.clicked.connect(self.populate_options)

    def populate_options(self):
        """
        Populate the UI with the current configuration settings.
        """
        file_cache = self.settings['file_cache']
        az_storage = self.settings['azstorage']
        libfuse = self.settings['libfuse']
        
        populate_widgets_from_settings(self._file_cache_mapping, file_cache)
        populate_widgets_from_settings(self._az_storage_mapping, az_storage)
        populate_widgets_from_settings(self._libfuse_mapping, libfuse)

        self.dropDown_azure_blobTier.setCurrentIndex(
            az_blob_tier.index(az_storage['tier'])
        )
        self.dropDown_fileCache_evictionPolicy.setCurrentIndex(
            file_cache_eviction_choices.index(file_cache['policy'])
        )

        if platform == 'win32':
            self.checkBox_libfuse_networkshare.setToolTip(
                'Runs as a network share - may improve performance when latency to cloud is high.'
            )
        else:
            self.checkBox_libfuse_networkshare.setEnabled(False)
            self.checkBox_libfuse_networkshare.setToolTip(
                'Network share is only supported on Windows'
            )

    def update_optional_az_storage(self):
        """
        Update the Azure storage settings from the UI values.
        """
        az_storage = self.settings['azstorage']        
        update_settings_from_widgets(self._az_storage_mapping, az_storage)
        self.settings['azstorage'] = az_storage

    def update_settings_from_ui_choices(self):
        """
        Update all settings from the UI values.
        """
        self.update_optional_az_storage()
        self.update_optional_file_cache()
        self.update_optional_libfuse()
