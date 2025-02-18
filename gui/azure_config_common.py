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

from PySide6.QtCore import Qt, QSettings
from PySide6.QtWidgets import QLineEdit, QFileDialog, QMessageBox
from PySide6.QtGui import QRegularExpressionValidator

# noinspection PyUnresolvedReferences
from __feature__ import snake_case, true_property


# import the custom class made from QtDesigner
from utils import set_path_validator, update_settings_from_widgets, populate_widgets_from_settings
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
    """
    def __init__(self, config_settings: dict):
        """
        Initialize the AzureSettingsWidget with the given configuration settings.

        Args:
            configSettings (dict): Configuration settings for Azure.
        """
        super().__init__()
        self.setup_ui(self)
        self.set_window_title('Azure Config Settings')
        self.my_window = QSettings('Cloudfuse', 'AzcWindow')
        self.settings = config_settings

        self._az_storage_mapping = {
            'account-key': self.lineEdit_azure_accountKey,
            'sas': self.lineEdit_azure_sasStorage,
            'account-name': self.lineEdit_azure_accountName,
            'container': self.lineEdit_azure_container,
            'endpoint': self.lineEdit_azure_endpoint,
            'appid': self.lineEdit_azure_msiAppID,
            'resid': self.lineEdit_azure_msiResourceID,
            'objid': self.lineEdit_azure_msiObjectID,
            'tenantid': self.lineEdit_azure_spnTenantID,
            'clientid': self.lineEdit_azure_spnClientID,
            'clientsecret': self.lineEdit_azure_spnClientSecret,
            'mode': self.dropDown_azure_modeSetting,
            'type': self.dropDown_azure_storageType,
        }
        self._libfuse_mapping = {
            'attribute-expiration-sec': self.spinBox_libfuse_attExp,
            'entry-expiration-sec': self.spinBox_libfuse_entExp,
            'negative-entry-expiration-sec': self.spinBox_libfuse_negEntryExp,
            'ignore-open-flags': self.checkBox_libfuse_ignoreAppend,
            'default-permission': self.dropDown_libfuse_permissions,
        }
        self._stream_mapping = {
            'file-caching': self.checkBox_streaming_fileCachingLevel,
            'block-size-mb': self.spinBox_streaming_blockSize,
            'buffer-size-mb': self.spinBox_streaming_buffSize,
            'max-buffers': self.spinBox_streaming_maxBuff,
        }
        self._file_cache_mapping = {
            'path': self.lineEdit_fileCache_path,
        }
        self._settings_mapping = {
            'allow-other': self.checkBox_multiUser,
            'nonempty': self.checkBox_nonEmptyDir,
            'foreground': self.checkBox_daemonForeground,
            'read-only': self.checkBox_readOnly,
        }

        self.init_window_size_pos()
        # Hide the pipeline mode groupbox depending on the default select is
        self.show_azure_mode_settings()
        self.show_mode_settings()
        self.populate_options()
        self._save_button_clicked = False

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
        # https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/resource-name-rules#microsoftstorage
        # Allow lowercase alphanumeric characters plus [-]
        self.lineEdit_azure_container.set_validator(
            QRegularExpressionValidator(r'^[a-z0-9-]*$', self)
        )
        # Allow alphanumeric characters plus [.,-,_]
        self.lineEdit_azure_accountName.set_validator(
            QRegularExpressionValidator(r'^[a-zA-Z0-9-._]*$', self)
        )

        set_path_validator(self.lineEdit_fileCache_path)

        self.lineEdit_azure_accountKey.set_echo_mode(
            QLineEdit.EchoMode.Password
        )
        self.lineEdit_azure_spnClientSecret.set_echo_mode(
            QLineEdit.EchoMode.Password
        )

    def update_az_storage(self):
        """
        Update the Azure storage settings from the UI choices.
        """
        az_storage = self.settings['azstorage']
        update_settings_from_widgets(self._az_storage_mapping, az_storage)
        self.settings['azstorage'] = az_storage

    def open_advanced(self):
        """
        Open the advanced settings window.
        """
        self.more_settings = AzureAdvancedSettingsWidget(self.settings)
        self.more_settings.set_window_modality(Qt.ApplicationModal)
        self.more_settings.show()

    def show_mode_settings(self):
        """
        Show file_cache or stream settings based on the selected pipeline.
        """
        self.hide_mode_boxes()
        components = self.settings['components']
        pipeline_index = self.dropDown_pipeline.current_index()
        components[1] = pipelineChoices[pipeline_index]
        if pipelineChoices[pipeline_index] == 'file_cache':
            self.groupbox_fileCache.set_visible(True)
        elif pipelineChoices[pipeline_index] == 'stream':
            self.groupbox_streaming.set_visible(True)
        self.settings['components'] = components

    def show_azure_mode_settings(self):
        """
        Show the appropriate Azure mode settings based on the selected mode.
        """
        self.hide_azure_boxes()
        mode_selection_index = self.dropDown_azure_modeSetting.current_index()
        # Azure mode group boxes
        if bucketModeChoices[mode_selection_index] == 'key':
            self.groupbox_accountKey.set_visible(True)
        elif bucketModeChoices[mode_selection_index] == 'sas':
            self.groupbox_sasStorage.set_visible(True)
        elif bucketModeChoices[mode_selection_index] == 'spn':
            self.groupbox_spn.set_visible(True)
        elif bucketModeChoices[mode_selection_index] == 'msi':
            self.groupbox_msi.set_visible(True)

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
        # reflect the index choices in human words without having to reference the UI.
        # Get the value in the settings and translate that to the equivalent index in the lists.
        self.dropDown_pipeline.set_current_index(
            pipelineChoices.index(self.settings['components'][1])
        )
        self.dropDown_libfuse_permissions.set_current_index(
            libfusePermissions.index(self.settings['libfuse']['default-permission'])
        )
        self.dropDown_azure_storageType.set_current_index(
            azStorageType.index(self.settings['azstorage']['type'])
        )
        self.dropDown_azure_modeSetting.set_current_index(
            bucketModeChoices.index(self.settings['azstorage']['mode'])
        )

        populate_widgets_from_settings(self._file_cache_mapping, file_cache)
        populate_widgets_from_settings(self._az_storage_mapping, az_storage)
        populate_widgets_from_settings(self._libfuse_mapping, libfuse)
        populate_widgets_from_settings(self._stream_mapping, stream)
        populate_widgets_from_settings(self._settings_mapping, self.settings)

    def get_file_dir_input(self):
        """
        Open a file dialog to select a directory and update the file cache path.
        """
        directory = str(QFileDialog.get_existing_directory())
        self.lineEdit_fileCache_path.set_text(f'{directory}')
        # Update the settings
        self.update_file_cache_path()

    def hide_mode_boxes(self):
        """
        Hide all mode group boxes.
        """
        self.groupbox_fileCache.set_visible(False)
        self.groupbox_streaming.set_visible(False)

    def hide_azure_boxes(self):
        """
        Hide all Azure mode group boxes.
        """
        self.groupbox_accountKey.set_visible(False)
        self.groupbox_sasStorage.set_visible(False)
        self.groupbox_spn.set_visible(False)
        self.groupbox_msi.set_visible(False)

    def reset_defaults(self):
        """
        Reset the settings to their default values.
        """
        # Reset these defaults
        check_choice = self.popup_double_check_reset()
        if check_choice == QMessageBox.Yes:
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
