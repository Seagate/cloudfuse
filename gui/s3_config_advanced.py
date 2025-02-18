"""
Defines the S3AdvancedSettingsWidget class for configuring advanced S3 settings.
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
from ui_s3_config_advanced import Ui_Form
from common_qt_functions import WidgetCustomFunctions

file_cache_eviction_choices = ['lru', 'lfu']


class S3AdvancedSettingsWidget(WidgetCustomFunctions, Ui_Form):
    """
    A widget for configuring advanced S3 settings.

    Attributes:
        settings (dict): Configuration settings for S3.
        my_window (QSettings): QSettings object for storing window state.
    """
    def __init__(self, configSettings: dict):
        """
        Initialize the S3AdvancedSettingsWidget with the given configuration settings.

        Args:
            configSettings (dict): Configuration settings for S3.
        """
        super().__init__()
        self.setupUi(self)
        self.my_window = QSettings('Cloudfuse', 'S3AdvancedWindow')
        self.settings = configSettings
        
        self._s3_storage_mapping = {
            'subdirectory': self.lineEdit_subdirectory,
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
        self.setWindowTitle('Advanced S3 Config Settings')
        self.populate_options()
        self._save_button_clicked = False

        set_path_validator(self.lineEdit_subdirectory)

        # Set up the signals
        self.button_okay.clicked.connect(self.exit_window)
        self.button_resetDefaultSettings.clicked.connect(self.populate_options)

    def populate_options(self):
        """
        Populate the UI with the current configuration settings.
        """
        file_cache = self.settings['file_cache']
        libfuse = self.settings['libfuse']
        s3_storage = self.settings['s3storage']

        populate_widgets_from_settings(self._file_cache_mapping, file_cache)
        populate_widgets_from_settings(self._s3_storage_mapping, s3_storage)
        populate_widgets_from_settings(self._libfuse_mapping, libfuse)
        
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

    def update_optional_s3_storage(self):
        """
        Update the optional S3 storage settings from the UI values.
        """
        s3_storage = self.settings['s3storage']
        update_settings_from_widgets(self._s3_storage_mapping, s3_storage)
        self.settings['s3storage'] = s3_storage

    def update_settings_from_ui_choices(self):
        """
        Update all settings from the UI values.
        """
        self.update_optional_file_cache()
        self.update_optional_libfuse()
        self.update_optional_s3_storage()
