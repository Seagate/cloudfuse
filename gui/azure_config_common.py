"""
Defines the AzureSettingsWidget class for configuring Azure settings.
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
from PySide6.QtCore import Qt, QSettings
from PySide6.QtWidgets import QLineEdit, QFileDialog
from PySide6.QtGui import QRegularExpressionValidator

# import the custom class made from QtDesigner
from ui_azure_config_common import Ui_Form
from azure_config_advanced import AzureAdvancedSettingsWidget
from common_qt_functions import WidgetCustomFunctions, DefaultSettingsManager

pipelineChoices = ['file_cache', 'stream', 'block_cache']
bucketModeChoices = ['key', 'sas', 'spn', 'msi']
azStorageType = ['block', 'adls']
libfusePermissions = [0o777, 0o666, 0o644, 0o444]


class AzureSettingsWidget(WidgetCustomFunctions, Ui_Form):
    """
    A widget for configuring Azure settings.

    Attributes:
        settings (dict): Configuration settings for Azure.
        my_window (QSettings): QSettings object for storing window state.
        save_button_clicked (bool): Flag to indicate if the save button was clicked.
    """
    def __init__(self, config_settings: dict):
        """
        Initialize the AzureSettingsWidget with the given configuration settings.

        Args:
            configSettings (dict): Configuration settings for Azure.
        """
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle('Azure Config Settings')
        self.my_window = QSettings('Cloudfuse', 'AzcWindow')
        self.settings = config_settings
        self.init_window_size_pos()
        # Hide the pipeline mode groupbox depending on the default select is
        self.show_azure_mode_settings()
        self.show_mode_settings()
        self.populate_options()
        self.save_button_clicked = False

        # Set up signals
        self.dropDown_pipeline.currentIndexChanged.connect(self.show_mode_settings)
        self.dropDown_azure_modeSetting.currentIndexChanged.connect(
            self.show_azure_mode_settings
        )
        self.button_browse.clicked.connect(self.get_file_dir_input)
        self.button_okay.clicked.connect(self.exit_window)
        self.button_advancedSettings.clicked.connect(self.open_advanced)
        self.button_resetDefaultSettings.clicked.connect(self.reset_defaults)

        # Documentation for the allowed characters for azure:
        #   https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/resource-name-rules#microsoftstorage
        # Allow lowercase alphanumeric characters plus [-]
        self.lineEdit_azure_container.setValidator(
            QRegularExpressionValidator(r'^[a-z0-9-]*$', self)
        )
        # Allow alphanumeric characters plus [.,-,_]
        self.lineEdit_azure_accountName.setValidator(
            QRegularExpressionValidator(r'^[a-zA-Z0-9-._]*$', self)
        )

        if platform == 'win32':
            # Windows directory and filename conventions:
            #   https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#file-and-directory-names
            # Disallow the following [<,>,.,",|,?,*] - note, we still need directory characters to declare a path
            self.lineEdit_fileCache_path.setValidator(
                QRegularExpressionValidator(r'^[^<>."|?\0*]*$', self)
            )
        else:
            # Allow anything BUT Nul
            # Note: Different versions of Python don't like the embedded null character, send in the raw string instead
            self.lineEdit_fileCache_path.setValidator(
                QRegularExpressionValidator(r'^[^\0]*$', self)
            )

        self.lineEdit_azure_accountKey.setEchoMode(
            QLineEdit.EchoMode.Password
        )
        self.lineEdit_azure_spnClientSecret.setEchoMode(
            QLineEdit.EchoMode.Password
        )

    # Set up slots

    def update_az_storage(self):
        """
        Update the Azure storage settings from the UI choices.
        """
        az_storage = self.settings['azstorage']
        az_storage['account-key'] = self.lineEdit_azure_accountKey.text()
        az_storage['sas'] = self.lineEdit_azure_sasStorage.text()
        az_storage['account-name'] = self.lineEdit_azure_accountName.text()
        az_storage['container'] = self.lineEdit_azure_container.text()
        az_storage['endpoint'] = self.lineEdit_azure_endpoint.text()
        az_storage['appid'] = self.lineEdit_azure_msiAppID.text()
        az_storage['resid'] = self.lineEdit_azure_msiResourceID.text()
        az_storage['objid'] = self.lineEdit_azure_msiObjectID.text()
        az_storage['tenantid'] = self.lineEdit_azure_spnTenantID.text()
        az_storage['clientid'] = self.lineEdit_azure_spnClientID.text()
        az_storage['clientsecret'] = self.lineEdit_azure_spnClientSecret.text()
        az_storage['type'] = azStorageType[
            self.dropDown_azure_storageType.currentIndex()
        ]
        az_storage['mode'] = bucketModeChoices[
            self.dropDown_azure_modeSetting.currentIndex()
        ]
        self.settings['azstorage'] = az_storage

    def open_advanced(self):
        """
        Open the advanced settings window.
        """
        more_settings = AzureAdvancedSettingsWidget(self.settings)
        more_settings.setWindowModality(Qt.ApplicationModal)
        more_settings.show()

    # ShowModeSettings will switch which groupbox is visiible: stream or file_cache
    #   the function also updates the internal components settings through QSettings
    #   There is one slot for the signal to be pointed at which is why showmodesettings is used.
    def show_mode_settings(self):
        """
        Show the appropriate mode settings based on the selected pipeline.
        """
        self.hide_mode_boxes()
        components = self.settings['components']
        pipeline_index = self.dropDown_pipeline.currentIndex()
        components[1] = pipelineChoices[pipeline_index]
        if pipelineChoices[pipeline_index] == 'file_cache':
            self.groupbox_fileCache.setVisible(True)
        elif pipelineChoices[pipeline_index] == 'stream':
            self.groupbox_streaming.setVisible(True)
        self.settings['components'] = components

    def show_azure_mode_settings(self):
        """
        Show the appropriate Azure mode settings based on the selected mode.
        """
        self.hide_azure_boxes()
        mode_selection_index = self.dropDown_azure_modeSetting.currentIndex()
        # Azure mode group boxes
        if bucketModeChoices[mode_selection_index] == 'key':
            self.groupbox_accountKey.setVisible(True)
        elif bucketModeChoices[mode_selection_index] == 'sas':
            self.groupbox_sasStorage.setVisible(True)
        elif bucketModeChoices[mode_selection_index] == 'spn':
            self.groupbox_spn.setVisible(True)
        elif bucketModeChoices[mode_selection_index] == 'msi':
            self.groupbox_msi.setVisible(True)

    # This widget will not display all the options in settings, only the ones written in the UI file.
    def populate_options(self):
        """
        Populate the UI with the current configuration settings.
        """
        file_cache = self.settings['file_cache']
        az_storage = self.settings['azstorage']
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

        self.set_checkbox_from_setting(
            self.checkBox_multiUser, self.settings['allow-other']
        )
        self.set_checkbox_from_setting(
            self.checkBox_nonEmptyDir, self.settings['nonempty']
        )
        self.set_checkbox_from_setting(
            self.checkBox_daemonForeground, self.settings['foreground']
        )
        self.set_checkbox_from_setting(self.checkBox_readOnly, self.settings['read-only'])
        self.set_checkbox_from_setting(
            self.checkBox_streaming_fileCachingLevel, stream['file-caching']
        )
        self.set_checkbox_from_setting(
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

        self.lineEdit_azure_accountKey.setText(az_storage['account-key'])
        self.lineEdit_azure_sasStorage.setText(az_storage['sas'])
        self.lineEdit_azure_accountName.setText(az_storage['account-name'])
        self.lineEdit_azure_container.setText(az_storage['container'])
        self.lineEdit_azure_endpoint.setText(az_storage['endpoint'])
        self.lineEdit_azure_msiAppID.setText(az_storage['appid'])
        self.lineEdit_azure_msiResourceID.setText(az_storage['resid'])
        self.lineEdit_azure_msiObjectID.setText(az_storage['objid'])
        self.lineEdit_azure_spnTenantID.setText(az_storage['tenantid'])
        self.lineEdit_azure_spnClientID.setText(az_storage['clientid'])
        self.lineEdit_azure_spnClientSecret.setText(az_storage['clientsecret'])
        self.lineEdit_fileCache_path.setText(file_cache['path'])

    def get_file_dir_input(self):
        """
        Open a file dialog to select a directory and update the file cache path.
        """
        directory = str(QFileDialog.getExistingDirectory())
        self.lineEdit_fileCache_path.setText(f'{directory}')
        # Update the settings
        self.update_file_cache_path()

    def hide_mode_boxes(self):
        """
        Hide all mode group boxes.
        """
        self.groupbox_fileCache.setVisible(False)
        self.groupbox_streaming.setVisible(False)

    def hide_azure_boxes(self):
        """
        Hide all Azure mode group boxes.
        """
        self.groupbox_accountKey.setVisible(False)
        self.groupbox_sasStorage.setVisible(False)
        self.groupbox_spn.setVisible(False)
        self.groupbox_msi.setVisible(False)

    def reset_defaults(self):
        """
        Reset the settings to their default values.
        """
        # Reset these defaults
        check_choice = self.popup_double_check_reset()
        if check_choice == QtWidgets.QMessageBox.Yes:
            DefaultSettingsManager.set_azure_settings(self, self.settings)
            DefaultSettingsManager.set_component_settings(self, self.settings)
            self.populate_options()

    def update_settings_from_ui_choices(self):
        """
        Update all settings from the UI values.
        """
        self.update_file_cache_path()
        self.update_libfuse()
        self.update_stream()
        self.update_az_storage()
        self.update_multi_user()
        self.update_non_emtpy_dir()
        self.update_read_only()
        self.update_daemon_foreground()
