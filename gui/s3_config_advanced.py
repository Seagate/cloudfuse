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
from PySide6.QtGui import QRegularExpressionValidator

# import the custom class made from QtDesigner
from ui_s3_config_advanced import Ui_Form
from common_qt_functions import WidgetCustomFunctions

file_cache_eviction_choices = ['lru', 'lfu']


class S3AdvancedSettingsWidget(WidgetCustomFunctions, Ui_Form):
    """
    A widget for configuring advanced S3 settings.

    Attributes:
        settings (dict): Configuration settings for S3.
        my_window (QSettings): QSettings object for storing window state.
        save_button_clicked (bool): Flag to indicate if the save button was clicked.
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
        self.init_window_size_pos()
        self.setWindowTitle('Advanced S3Cloud Config Settings')
        self.populate_options()
        self.save_button_clicked = False

        if platform == 'win32':
            # Windows directory and filename conventions:
            #   https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#file-and-directory-names
            # Disallow the following [<,>,.,",|,?,*] - note, we still need directory characters to declare a path
            self.lineEdit_subdirectory.setValidator(
                QRegularExpressionValidator(r'^[^<>."|?\0*]*$', self)
            )
        else:
            # Allow anything BUT Nul
            # Note: Different versions of Python don't like the embedded null character, send in the raw string instead
            self.lineEdit_subdirectory.setValidator(
                QRegularExpressionValidator(r'^[^\0]*$', self)
            )

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

        # The index of file_cache_eviction is matched with the default
        #   index values in the ui code, so translate the value from settings to index number
        policy_index = file_cache_eviction_choices.index(file_cache['policy'])
        self.dropDown_fileCache_evictionPolicy.setCurrentIndex(policy_index)

        self.set_checkbox_from_setting(
            self.checkBox_libfuse_disableWriteback, libfuse['disable-writeback-cache']
        )
        self.set_checkbox_from_setting(
            self.checkBox_libfuse_networkshare, libfuse['network-share']
        )
        self.set_checkbox_from_setting(
            self.checkBox_fileCache_allowNonEmptyTmp, file_cache['allow-non-empty-temp']
        )
        self.set_checkbox_from_setting(
            self.checkBox_fileCache_policyLogs, file_cache['policy-trace']
        )
        self.set_checkbox_from_setting(
            self.checkBox_fileCache_createEmptyFile, file_cache['create-empty-file']
        )
        self.set_checkbox_from_setting(
            self.checkBox_fileCache_cleanupStart, file_cache['cleanup-on-start']
        )
        self.set_checkbox_from_setting(
            self.checkBox_fileCache_offloadIO, file_cache['offload-io']
        )
        self.set_checkbox_from_setting(
            self.checkBox_fileCache_syncToFlush, file_cache['sync-to-flush']
        )

        self.spinBox_fileCache_evictionTimeout.setValue(
            file_cache['timeout-sec'])
        self.spinBox_fileCache_maxEviction.setValue(file_cache['max-eviction'])
        self.spinBox_fileCache_maxCacheSize.setValue(file_cache['max-size-mb'])
        self.spinBox_fileCache_evictMaxThresh.setValue(
            file_cache['high-threshold'])
        self.spinBox_fileCache_evictMinThresh.setValue(
            file_cache['low-threshold'])
        self.spinBox_fileCache_refreshSec.setValue(file_cache['refresh-sec'])
        self.spinBox_libfuse_maxFuseThreads.setValue(
            libfuse['max-fuse-threads'])

        self.lineEdit_subdirectory.setText(s3_storage['subdirectory'])

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
        s3_storage['subdirectory'] = self.lineEdit_subdirectory.text()
        self.settings['s3storage'] = s3_storage

    def update_settings_from_ui_choices(self):
        """
        Update all settings from the UI values.
        """
        self.update_optional_file_cache()
        self.update_optional_libfuse()
        self.update_optional_s3_storage()
