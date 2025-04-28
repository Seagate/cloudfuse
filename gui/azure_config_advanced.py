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
from PySide6 import QtGui
# import the custom class made from QtDesigner
from ui_azure_config_advanced import Ui_Form
from common_qt_functions import widgetCustomFunctions

az_blob_tier = ['none','hot','cool','archive']

class azureAdvancedSettingsWidget(widgetCustomFunctions, Ui_Form):
    def __init__(self,configSettings):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle('Advanced Azure Config Settings')
        self.settings = configSettings
        self.myWindow = QSettings('Cloudfuse', 'AzAdvancedWindow')
        self.initWindowSizePos()
        self.populateOptions()
        self.saveButtonClicked = False

        if platform == 'win32':
            # Windows directory and filename conventions:
            #   https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#file-and-directory-names
            # Disallow the following [<,>,.,",|,?,*] - note, we still need directory characters to declare a path
            self.lineEdit_azure_subDirectory.setValidator(QtGui.QRegularExpressionValidator(r'^[^<>."|?\0*]*$',self))
        else:
            # Allow anything BUT Nul
            # Note: Different versions of Python don't like the embedded null character, send in the raw string instead
            self.lineEdit_azure_subDirectory.setValidator(QtGui.QRegularExpressionValidator(r'^[^\0]*$',self))

        # Set up the signals
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_resetDefaultSettings.clicked.connect(self.populateOptions)


    def populateOptions(self):
        fileCache = self.settings['file_cache']
        azStorage = self.settings['azstorage']
        libfuse = self.settings['libfuse']

        self.setCheckboxFromSetting(self.checkBox_libfuse_disableWriteback,libfuse['disable-writeback-cache'])
        self.setCheckboxFromSetting(self.checkBox_libfuse_networkshare, libfuse['network-share'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_allowNonEmptyTmp,fileCache['allow-non-empty-temp'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_policyLogs,fileCache['policy-trace'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_createEmptyFile,fileCache['create-empty-file'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_cleanupStart,fileCache['cleanup-on-start'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_offloadIO,fileCache['offload-io'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_syncToFlush, fileCache['sync-to-flush'])
        self.setCheckboxFromSetting(self.checkBox_azure_useHttp,azStorage['use-http'])
        self.setCheckboxFromSetting(self.checkBox_azure_validateMd5,azStorage['validate-md5'])
        self.setCheckboxFromSetting(self.checkBox_azure_updateMd5,azStorage['update-md5'])
        self.setCheckboxFromSetting(self.checkBox_azure_failUnsupportedOps, azStorage['fail-unsupported-op'])
        self.setCheckboxFromSetting(self.checkBox_azure_sdkTrace,azStorage['sdk-trace'])
        self.setCheckboxFromSetting(self.checkBox_azure_virtualDirectory,azStorage['virtual-directory'])
        self.setCheckboxFromSetting(self.checkBox_azure_disableCompression,azStorage['disable-compression'])

        self.spinBox_fileCache_evictionTimeout.setValue(fileCache['timeout-sec'])
        self.spinBox_fileCache_maxEviction.setValue(fileCache['max-eviction'])
        self.spinBox_fileCache_maxCacheSize.setValue(fileCache['max-size-mb'])
        self.spinBox_fileCache_evictMaxThresh.setValue(fileCache['high-threshold'])
        self.spinBox_fileCache_evictMinThresh.setValue(fileCache['low-threshold'])
        self.spinBox_fileCache_refreshSec.setValue(fileCache['refresh-sec'])
        self.spinBox_azure_blockSize.setValue(azStorage['block-size-mb'])
        self.spinBox_azure_maxConcurrency.setValue(azStorage['max-concurrency'])
        self.spinBox_azure_blockOnMount.setValue(azStorage['block-list-on-mount-sec'])
        self.spinBox_azure_maxRetries.setValue(azStorage['max-retries'])
        self.spinBox_azure_maxRetryTimeout.setValue(azStorage['max-retry-timeout-sec'])
        self.spinBox_azure_retryBackoff.setValue(azStorage['retry-backoff-sec'])
        self.spinBox_azure_maxRetryDelay.setValue(azStorage['max-retry-delay-sec'])
        self.spinBox_libfuse_maxFuseThreads.setValue(libfuse['max-fuse-threads'])

        self.lineEdit_azure_aadEndpoint.setText(azStorage['aadendpoint'])
        self.lineEdit_azure_subDirectory.setText(azStorage['subdirectory'])
        self.lineEdit_azure_httpProxy.setText(azStorage['http-proxy'])
        self.lineEdit_azure_httpsProxy.setText(azStorage['https-proxy'])
        self.lineEdit_azure_authResource.setText(azStorage['auth-resource'])

        self.dropDown_azure_blobTier.setCurrentIndex(az_blob_tier.index(azStorage['tier']))

        if platform == 'win32':
            self.checkBox_libfuse_networkshare.setToolTip('Runs as a network share - may improve performance when latency to cloud is high.')
        else:
            self.checkBox_libfuse_networkshare.setEnabled(False)
            self.checkBox_libfuse_networkshare.setToolTip('Network share is only supported on Windows')

    def updateOptionalAzStorage(self):
        azStorage = self.settings['azstorage']
        azStorage['block-size-mb'] = self.spinBox_azure_blockSize.value()
        azStorage['max-concurrency'] = self.spinBox_azure_maxConcurrency.value()
        azStorage['block-list-on-mount-sec'] = self.spinBox_azure_blockOnMount.value()
        azStorage['max-retries'] = self.spinBox_azure_maxRetries.value()
        azStorage['max-retry-timeout-sec'] = self.spinBox_azure_maxRetryTimeout.value()
        azStorage['retry-backoff-sec'] = self.spinBox_azure_retryBackoff.value()
        azStorage['max-retry-delay-sec'] = self.spinBox_azure_maxRetryDelay.value()

        azStorage['use-http'] = self.checkBox_azure_useHttp.isChecked()
        azStorage['validate-md5'] = self.checkBox_azure_validateMd5.isChecked()
        azStorage['update-md5'] = self.checkBox_azure_updateMd5.isChecked()
        azStorage['fail-unsupported-op'] = self.checkBox_azure_failUnsupportedOps.isChecked()
        azStorage['sdk-trace'] = self.checkBox_azure_sdkTrace.isChecked()
        azStorage['virtual-directory'] = self.checkBox_azure_virtualDirectory.isChecked()
        azStorage['disable-compression'] = self.checkBox_azure_disableCompression.isChecked()

        azStorage['aadendpoint'] = self.lineEdit_azure_aadEndpoint.text()
        azStorage['subdirectory'] = self.lineEdit_azure_subDirectory.text()
        azStorage['http-proxy'] = self.lineEdit_azure_httpProxy.text()
        azStorage['https-proxy'] = self.lineEdit_azure_httpsProxy.text()
        azStorage['auth-resource'] = self.lineEdit_azure_authResource.text()

        azStorage['tier'] = az_blob_tier[self.dropDown_azure_blobTier.currentIndex()]
        self.settings['azstorage'] = azStorage

    def updateSettingsFromUIChoices(self):
        self.updateOptionalAzStorage()
        self.updateOptionalFileCache()
        self.updateOptionalLibfuse()
