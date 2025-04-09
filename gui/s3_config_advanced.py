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
from ui_s3_config_advanced import Ui_Form
from common_qt_functions import widgetCustomFunctions

class s3AdvancedSettingsWidget(widgetCustomFunctions, Ui_Form):
    def __init__(self,configSettings):
        super().__init__()
        self.setupUi(self)
        self.myWindow = QSettings('Cloudfuse', 'S3AdvancedWindow')
        self.settings = configSettings
        self.initWindowSizePos()
        self.setWindowTitle('Advanced S3Cloud Config Settings')
        self.populateOptions()
        self.saveButtonClicked = False

        if platform == 'win32':
            # Windows directory and filename conventions:
            #   https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#file-and-directory-names
            # Disallow the following [<,>,.,",|,?,*] - note, we still need directory characters to declare a path
            self.lineEdit_subdirectory.setValidator(QtGui.QRegularExpressionValidator(r'^[^<>."|?\0*]*$',self))
        else:
            # Allow anything BUT Nul
            # Note: Different versions of Python don't like the embedded null character, send in the raw string instead
            self.lineEdit_subdirectory.setValidator(QtGui.QRegularExpressionValidator(r'^[^\0]*$',self))


        # Set up the signals
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_resetDefaultSettings.clicked.connect(self.populateOptions)

    def populateOptions(self):
        fileCache = self.settings['file_cache']
        libfuse = self.settings['libfuse']
        s3Storage = self.settings['s3storage']

        self.setCheckboxFromSetting(self.checkBox_libfuse_disableWriteback, libfuse['disable-writeback-cache'])
        self.setCheckboxFromSetting(self.checkBox_libfuse_networkshare, libfuse['network-share'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_allowNonEmptyTmp,fileCache['allow-non-empty-temp'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_policyLogs,fileCache['policy-trace'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_createEmptyFile,fileCache['create-empty-file'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_cleanupStart,fileCache['cleanup-on-start'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_offloadIO,fileCache['offload-io'])
        self.setCheckboxFromSetting(self.checkBox_fileCache_syncToFlush, fileCache['sync-to-flush'])

        self.spinBox_fileCache_evictionTimeout.setValue(fileCache['timeout-sec'])
        self.spinBox_fileCache_maxEviction.setValue(fileCache['max-eviction'])
        self.spinBox_fileCache_maxCacheSize.setValue(fileCache['max-size-mb'])
        self.spinBox_fileCache_evictMaxThresh.setValue(fileCache['high-threshold'])
        self.spinBox_fileCache_evictMinThresh.setValue(fileCache['low-threshold'])
        self.spinBox_fileCache_refreshSec.setValue(fileCache['refresh-sec'])
        self.spinBox_libfuse_maxFuseThreads.setValue(libfuse['max-fuse-threads'])

        self.lineEdit_subdirectory.setText(s3Storage['subdirectory'])

        if platform == 'win32':
            self.checkBox_libfuse_networkshare.setToolTip('Runs as a network share - may improve performance when latency to cloud is high.')
        else:
            self.checkBox_libfuse_networkshare.setEnabled(False)
            self.checkBox_libfuse_networkshare.setToolTip('Network share is only supported on Windows')

    def updateOptionalS3Storage(self):
        s3Storage = self.settings['s3storage']
        s3Storage['subdirectory'] = self.lineEdit_subdirectory.text()
        self.settings['s3storage'] = s3Storage

    def updateSettingsFromUIChoices(self):
        self.updateOptionalFileCache()
        self.updateOptionalLibfuse()
        self.updateOptionalS3Storage()
