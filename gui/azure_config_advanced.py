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
from PySide6.QtGui import QRegularExpressionValidator

# import the custom class made from QtDesigner
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
        self.init_window_size_pos()
        self.populate_options()
        self.save_button_clicked = False

        if platform == 'win32':
            # Windows directory and filename conventions:
            #   https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#file-and-directory-names
            # Disallow the following [<,>,.,",|,?,*] - note, we still need directory characters to declare a path
            self.lineEdit_azure_subDirectory.setValidator(
                QRegularExpressionValidator(r'^[^<>."|?\0*]*$', self)
            )
        else:
            # Allow anything BUT Nul
            # Note: Different versions of Python don't like the embedded null character, send in the raw string instead
            self.lineEdit_azure_subDirectory.setValidator(
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
        az_storage = self.settings['azstorage']
        libfuse = self.settings['libfuse']

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
        self.set_checkbox_from_setting(
            self.checkBox_azure_useHttp, az_storage['use-http'])
        self.set_checkbox_from_setting(
            self.checkBox_azure_validateMd5, az_storage['validate-md5']
        )
        self.set_checkbox_from_setting(
            self.checkBox_azure_updateMd5, az_storage['update-md5']
        )
        self.set_checkbox_from_setting(
            self.checkBox_azure_failUnsupportedOps, az_storage['fail-unsupported-op']
        )
        self.set_checkbox_from_setting(
            self.checkBox_azure_sdkTrace, az_storage['sdk-trace']
        )
        self.set_checkbox_from_setting(
            self.checkBox_azure_virtualDirectory, az_storage['virtual-directory']
        )
        self.set_checkbox_from_setting(
            self.checkBox_azure_disableCompression, az_storage['disable-compression']
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
        self.spinBox_azure_blockSize.setValue(az_storage['block-size-mb'])
        self.spinBox_azure_maxConcurrency.setValue(
            az_storage['max-concurrency'])
        self.spinBox_azure_blockOnMount.setValue(
            az_storage['block-list-on-mount-sec'])
        self.spinBox_azure_maxRetries.setValue(az_storage['max-retries'])
        self.spinBox_azure_maxRetryTimeout.setValue(
            az_storage['max-retry-timeout-sec'])
        self.spinBox_azure_retryBackoff.setValue(
            az_storage['retry-backoff-sec'])
        self.spinBox_azure_maxRetryDelay.setValue(
            az_storage['max-retry-delay-sec'])
        self.spinBox_libfuse_maxFuseThreads.setValue(
            libfuse['max-fuse-threads'])

        self.lineEdit_azure_aadEndpoint.setText(az_storage['aadendpoint'])
        self.lineEdit_azure_subDirectory.setText(az_storage['subdirectory'])
        self.lineEdit_azure_httpProxy.setText(az_storage['http-proxy'])
        self.lineEdit_azure_httpsProxy.setText(az_storage['https-proxy'])
        self.lineEdit_azure_authResource.setText(az_storage['auth-resource'])

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
        az_storage['block-size-mb'] = self.spinBox_azure_blockSize.value()
        az_storage['max-concurrency'] = self.spinBox_azure_maxConcurrency.value()
        az_storage['block-list-on-mount-sec'] = self.spinBox_azure_blockOnMount.value()
        az_storage['max-retries'] = self.spinBox_azure_maxRetries.value()
        az_storage['max-retry-timeout-sec'] = self.spinBox_azure_maxRetryTimeout.value()
        az_storage['retry-backoff-sec'] = self.spinBox_azure_retryBackoff.value()
        az_storage['max-retry-delay-sec'] = self.spinBox_azure_maxRetryDelay.value()

        az_storage['use-http'] = self.checkBox_azure_useHttp.isChecked()
        az_storage['validate-md5'] = self.checkBox_azure_validateMd5.isChecked()
        az_storage['update-md5'] = self.checkBox_azure_updateMd5.isChecked()
        az_storage['fail-unsupported-op'] = (
            self.checkBox_azure_failUnsupportedOps.isChecked()
        )
        az_storage['sdk-trace'] = self.checkBox_azure_sdkTrace.isChecked()
        az_storage['virtual-directory'] = (
            self.checkBox_azure_virtualDirectory.isChecked()
        )
        az_storage['disable-compression'] = (
            self.checkBox_azure_disableCompression.isChecked()
        )

        az_storage['aadendpoint'] = self.lineEdit_azure_aadEndpoint.text()
        az_storage['subdirectory'] = self.lineEdit_azure_subDirectory.text()
        az_storage['http-proxy'] = self.lineEdit_azure_httpProxy.text()
        az_storage['https-proxy'] = self.lineEdit_azure_httpsProxy.text()
        az_storage['auth-resource'] = self.lineEdit_azure_authResource.text()

        az_storage['tier'] = az_blob_tier[self.dropDown_azure_blobTier.currentIndex()]
        self.settings['azstorage'] = az_storage

    def update_settings_from_ui_choices(self):
        """
        Update all settings from the UI values.
        """
        self.update_optional_az_storage()
        self.update_optional_file_cache()
        self.update_optional_libfuse()
