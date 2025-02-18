"""
Defines the S3SettingsWidget class for configuring S3 settings.
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
from PySide6.QtGui import QRegularExpressionValidator
from PySide6.QtWidgets import QLineEdit, QFileDialog, QMessageBox

# import the custom class made from QtDesigner
from utils import set_path_validator, populate_widgets_from_settings, update_settings_from_widgets
from ui_s3_config_common import Ui_Form
from s3_config_advanced import S3AdvancedSettingsWidget
from common_qt_functions import WidgetCustomFunctions, DefaultSettingsManager

pipelineChoices = ['file_cache', 'stream', 'block_cache']
libfusePermissions = [0o777, 0o666, 0o644, 0o444]


class S3SettingsWidget(WidgetCustomFunctions, Ui_Form):
    """
    A widget for configuring S3 settings.

    Attributes:
        settings (dict): Configuration settings for S3.
        my_window (QSettings): QSettings object for storing window state.
    """
    def __init__(self, configSettings: dict):
        """
        Initialize the S3SettingsWidget with the given configuration settings.

        Args:
            configSettings (dict): Configuration settings for S3.
        """
        super().__init__()
        self.setupUi(self)
        self.my_window = QSettings('Cloudfuse', 's3Window')
        self.init_window_size_pos()
        self.setWindowTitle('S3 Config Settings')
        self.settings = configSettings

        self._s3_storage_mapping = {
            'bucket-name': self.lineEdit_bucketName,
            'endpoint': self.lineEdit_endpoint,
            'secret-key': self.lineEdit_secretKey,
            'key-id': self.lineEdit_accessKey,
            'region': self.lineEdit_region,
            'path': self.lineEdit_fileCache_path,
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

        self.populate_options()
        self.show_mode_settings()
        self.save_button_clicked = False

        # S3 naming conventions:
        #   https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html
        # Allow lowercase alphanumeric characters plus [.,-]
        self.lineEdit_bucketName.setValidator(
            QRegularExpressionValidator(r'^[a-z0-9-.]*$', self)
        )
        # Allow alphanumeric characters plus [-,_]
        self.lineEdit_region.setValidator(
            QRegularExpressionValidator(r'^[a-zA-Z0-9-_]*$', self)
        )

        set_path_validator(self.lineEdit_fileCache_path)

        # Hide sensitive data QLineEdit.EchoMode.PasswordEchoOnEdit
        self.lineEdit_accessKey.setEchoMode(
            QLineEdit.EchoMode.Password)
        self.lineEdit_secretKey.setEchoMode(
            QLineEdit.EchoMode.Password)

        # Set up signals for buttons
        self.dropDown_pipeline.currentIndexChanged.connect(
            self.show_mode_settings)
        self.button_browse.clicked.connect(self.get_file_dir_input)
        self.button_okay.clicked.connect(self.exit_window)
        self.button_advancedSettings.clicked.connect(self.open_advanced)
        self.button_resetDefaultSettings.clicked.connect(self.reset_defaults)

    # Set up slots for the signals:

    # To open the advanced widget, make an instance, so self.moresettings was chosen.
    #   self.moresettings does not have anything to do with the QSettings package that is seen throughout this code
    def open_advanced(self):
        """
        Open the advanced settings widget.
        """
        self.more_settings = S3AdvancedSettingsWidget(self.settings)
        self.more_settings.setWindowModality(Qt.ApplicationModal)
        self.more_settings.show()

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

    def get_file_dir_input(self):
        """
        Open a file dialog to select a directory.
        """
        directory = str(QFileDialog.getExistingDirectory())
        self.lineEdit_fileCache_path.setText(f'{directory}')

    def hide_mode_boxes(self):
        """
        Hide all mode group boxes.
        """
        self.groupbox_fileCache.setVisible(False)
        self.groupbox_streaming.setVisible(False)

        # Update S3Storage re-writes everything in the S3Storage dictionary for the same reason update libfuse does.

    def update_s3_storage(self):
        """
        Update the S3 storage settings from the UI choices.
        """
        s3_storage = self.settings['s3storage']
        update_settings_from_widgets(self._s3_storage_mapping, s3_storage)
        self.settings['s3storage'] = s3_storage

    # This widget will not display all the options in settings, only the ones written in the UI file.
    def populate_options(self):
        """
        Populate the UI with the current configuration settings.
        """
        file_cache = self.settings['file_cache']
        s3_storage = self.settings['s3storage']
        libfuse = self.settings['libfuse']
        stream = self.settings['stream']

        populate_widgets_from_settings(self._file_cache_mapping, file_cache)
        populate_widgets_from_settings(self._s3_storage_mapping, s3_storage)
        populate_widgets_from_settings(self._libfuse_mapping, libfuse)
        populate_widgets_from_settings(self._stream_mapping, stream)
        populate_widgets_from_settings(self._settings_mapping, self.settings)

        # Must populate the group boxes since those don't populate above.
        self.dropDown_pipeline.setCurrentIndex(
            pipelineChoices.index(self.settings['components'][1])
        )
        self.dropDown_libfuse_permissions.setCurrentIndex(
            libfusePermissions.index(libfuse['default-permission'])
        )

    def reset_defaults(self):
        """
        Reset the settings to their default values.
        """
        # Reset these defaults
        check_choice = self.popup_double_check_reset()
        if check_choice == QMessageBox.Yes:
            DefaultSettingsManager.set_s3_settings(self, self.settings)
            DefaultSettingsManager.set_component_settings(self, self.settings)
            self.populate_options()

    def update_settings_from_ui_choices(self):
        """
        Update all settings from the UI choices.
        """
        self.update_file_cache_path()
        self.update_libfuse()
        self.update_read_only()
        self.update_daemon_foreground()
        self.update_multi_user()
        self.update_non_emtpy_dir()
        self.update_stream()
        self.update_s3_storage()
