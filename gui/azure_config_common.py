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
from PySide6.QtCore import Qt, QSettings
from PySide6 import QtWidgets, QtGui

# import the custom class made from QtDesigner
from ui_azure_config_common import Ui_Form
from azure_config_advanced import azureAdvancedSettingsWidget
from common_qt_functions import widgetCustomFunctions, defaultSettingsManager

pipelineChoices = ['file_cache', 'stream', 'block_cache']
bucketModeChoices = ['key', 'sas', 'spn', 'msi']
azStorageType = ['block', 'adls']
libfusePermissions = [0o777, 0o666, 0o644, 0o444]


class azureSettingsWidget(widgetCustomFunctions, Ui_Form):
    def __init__(self, configSettings):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle('Azure Config Settings')
        self.myWindow = QSettings('Cloudfuse', 'AzcWindow')
        self.settings = configSettings
        self.initWindowSizePos()
        # Hide the pipeline mode groupbox depending on the default select is
        self.showAzureModeSettings()
        self.showModeSettings()
        self.populateOptions()
        self.saveButtonClicked = False

        # Set up signals
        self.dropDown_pipeline.currentIndexChanged.connect(self.showModeSettings)
        self.dropDown_azure_modeSetting.currentIndexChanged.connect(
            self.showAzureModeSettings
        )
        self.button_browse.clicked.connect(self.getFileDirInput)
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_advancedSettings.clicked.connect(self.openAdvanced)
        self.button_resetDefaultSettings.clicked.connect(self.resetDefaults)

        # Documentation for the allowed characters for azure:
        #   https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/resource-name-rules#microsoftstorage
        # Allow lowercase alphanumeric characters plus [-]
        self.lineEdit_azure_container.setValidator(
            QtGui.QRegularExpressionValidator(r'^[a-z0-9-]*$', self)
        )
        # Allow alphanumeric characters plus [.,-,_]
        self.lineEdit_azure_accountName.setValidator(
            QtGui.QRegularExpressionValidator(r'^[a-zA-Z0-9-._]*$', self)
        )

        if platform == 'win32':
            # Windows directory and filename conventions:
            #   https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#file-and-directory-names
            # Disallow the following [<,>,.,",|,?,*] - note, we still need directory characters to declare a path
            self.lineEdit_fileCache_path.setValidator(
                QtGui.QRegularExpressionValidator(r'^[^<>."|?\0*]*$', self)
            )
        else:
            # Allow anything BUT Nul
            # Note: Different versions of Python don't like the embedded null character, send in the raw string instead
            self.lineEdit_fileCache_path.setValidator(
                QtGui.QRegularExpressionValidator(r'^[^\0]*$', self)
            )

        self.lineEdit_azure_accountKey.setEchoMode(
            QtWidgets.QLineEdit.EchoMode.Password
        )
        self.lineEdit_azure_spnClientSecret.setEchoMode(
            QtWidgets.QLineEdit.EchoMode.Password
        )

    # Set up slots

    def updateAzStorage(self):
        azStorage = self.settings['azstorage']
        azStorage['account-key'] = self.lineEdit_azure_accountKey.text()
        azStorage['sas'] = self.lineEdit_azure_sasStorage.text()
        azStorage['account-name'] = self.lineEdit_azure_accountName.text()
        azStorage['container'] = self.lineEdit_azure_container.text()
        azStorage['endpoint'] = self.lineEdit_azure_endpoint.text()
        azStorage['appid'] = self.lineEdit_azure_msiAppID.text()
        azStorage['resid'] = self.lineEdit_azure_msiResourceID.text()
        azStorage['objid'] = self.lineEdit_azure_msiObjectID.text()
        azStorage['tenantid'] = self.lineEdit_azure_spnTenantID.text()
        azStorage['clientid'] = self.lineEdit_azure_spnClientID.text()
        azStorage['clientsecret'] = self.lineEdit_azure_spnClientSecret.text()
        azStorage['type'] = azStorageType[
            self.dropDown_azure_storageType.currentIndex()
        ]
        azStorage['mode'] = bucketModeChoices[
            self.dropDown_azure_modeSetting.currentIndex()
        ]
        self.settings['azstorage'] = azStorage

    def openAdvanced(self):
        self.moreSettings = azureAdvancedSettingsWidget(self.settings)
        self.moreSettings.setWindowModality(Qt.ApplicationModal)
        self.moreSettings.show()

    # ShowModeSettings will switch which groupbox is visiible: stream or file_cache
    #   the function also updates the internal components settings through QSettings
    #   There is one slot for the signal to be pointed at which is why showmodesettings is used.
    def showModeSettings(self):
        self.hideModeBoxes()
        components = self.settings['components']
        pipelineIndex = self.dropDown_pipeline.currentIndex()
        components[1] = pipelineChoices[pipelineIndex]
        if pipelineChoices[pipelineIndex] == 'file_cache':
            self.groupbox_fileCache.setVisible(True)
        elif pipelineChoices[pipelineIndex] == 'stream':
            self.groupbox_streaming.setVisible(True)
        self.settings['components'] = components

    def showAzureModeSettings(self):
        self.hideAzureBoxes()
        modeSelectionIndex = self.dropDown_azure_modeSetting.currentIndex()
        # Azure mode group boxes
        if bucketModeChoices[modeSelectionIndex] == 'key':
            self.groupbox_accountKey.setVisible(True)
        elif bucketModeChoices[modeSelectionIndex] == 'sas':
            self.groupbox_sasStorage.setVisible(True)
        elif bucketModeChoices[modeSelectionIndex] == 'spn':
            self.groupbox_spn.setVisible(True)
        elif bucketModeChoices[modeSelectionIndex] == 'msi':
            self.groupbox_msi.setVisible(True)

    # This widget will not display all the options in settings, only the ones written in the UI file.
    def populateOptions(self):
        fileCache = self.settings['file_cache']
        azStorage = self.settings['azstorage']
        libfuse = self.settings['libfuse']
        stream = self.settings['stream']

        # The QCombo (dropdown selection) uses indices to determine the value to show the user. The pipelineChoices, libfusePermissions, azStorage and bucketMode
        #   reflect the index choices in human words without having to reference the UI.
        #   Get the value in the settings and translate that to the equivalent index in the lists.
        self.dropDown_pipeline.setCurrentIndex(
            pipelineChoices.index(self.settings['components'][1])
        )
        self.dropDown_libfuse_permissions.setCurrentIndex(
            libfusePermissions.index(self.settings['libfuse']['default-permission'])
        )
        self.dropDown_azure_storageType.setCurrentIndex(
            azStorageType.index(self.settings['azstorage']['type'])
        )
        self.dropDown_azure_modeSetting.setCurrentIndex(
            bucketModeChoices.index(self.settings['azstorage']['mode'])
        )

        self.setCheckboxFromSetting(
            self.checkBox_multiUser, self.settings['allow-other']
        )
        self.setCheckboxFromSetting(
            self.checkBox_nonEmptyDir, self.settings['nonempty']
        )
        self.setCheckboxFromSetting(
            self.checkBox_daemonForeground, self.settings['foreground']
        )
        self.setCheckboxFromSetting(self.checkBox_readOnly, self.settings['read-only'])
        self.setCheckboxFromSetting(
            self.checkBox_streaming_fileCachingLevel, stream['file-caching']
        )
        self.setCheckboxFromSetting(
            self.checkBox_libfuse_ignoreAppend, libfuse['ignore-open-flags']
        )

        # Spinbox automatically sanitizes inputs for decimal values only, so no need to check for the appropriate data type.
        self.spinBox_libfuse_attExp.setValue(libfuse['attribute-expiration-sec'])
        self.spinBox_libfuse_entExp.setValue(libfuse['entry-expiration-sec'])
        self.spinBox_libfuse_negEntryExp.setValue(
            libfuse['negative-entry-expiration-sec']
        )
        self.spinBox_streaming_blockSize.setValue(stream['block-size-mb'])
        self.spinBox_streaming_buffSize.setValue(stream['buffer-size-mb'])
        self.spinBox_streaming_maxBuff.setValue(stream['max-buffers'])

        # There is no sanitizing for lineEdit at the moment, the GUI depends on the user being correct.

        self.lineEdit_azure_accountKey.setText(azStorage['account-key'])
        self.lineEdit_azure_sasStorage.setText(azStorage['sas'])
        self.lineEdit_azure_accountName.setText(azStorage['account-name'])
        self.lineEdit_azure_container.setText(azStorage['container'])
        self.lineEdit_azure_endpoint.setText(azStorage['endpoint'])
        self.lineEdit_azure_msiAppID.setText(azStorage['appid'])
        self.lineEdit_azure_msiResourceID.setText(azStorage['resid'])
        self.lineEdit_azure_msiObjectID.setText(azStorage['objid'])
        self.lineEdit_azure_spnTenantID.setText(azStorage['tenantid'])
        self.lineEdit_azure_spnClientID.setText(azStorage['clientid'])
        self.lineEdit_azure_spnClientSecret.setText(azStorage['clientsecret'])
        self.lineEdit_fileCache_path.setText(fileCache['path'])

    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.lineEdit_fileCache_path.setText('{}'.format(directory))
        # Update the settings
        self.updateFileCachePath()

    def hideModeBoxes(self):
        self.groupbox_fileCache.setVisible(False)
        self.groupbox_streaming.setVisible(False)

    def hideAzureBoxes(self):
        self.groupbox_accountKey.setVisible(False)
        self.groupbox_sasStorage.setVisible(False)
        self.groupbox_spn.setVisible(False)
        self.groupbox_msi.setVisible(False)

    def resetDefaults(self):
        # Reset these defaults
        checkChoice = self.popupDoubleCheckReset()
        if checkChoice == QtWidgets.QMessageBox.Yes:
            defaultSettingsManager.setAzureSettings(self, self.settings)
            defaultSettingsManager.setComponentSettings(self, self.settings)
            self.populateOptions()

    def updateSettingsFromUIChoices(self):
        self.updateFileCachePath()
        self.updateLibfuse()
        self.updateStream()
        self.updateAzStorage()
        self.updateMultiUser()
        self.updateNonEmtpyDir()
        self.updateReadOnly()
        self.updateDaemonForeground()
