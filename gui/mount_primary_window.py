"""
Defines the FUSEWindow class for managing the Cloudfuse application.
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

import os
import subprocess
from shutil import which
from sys import platform

# Import QT libraries
from PySide6.QtGui import QTextCursor
from PySide6.QtCore import QSettings, Qt, QTimer
from PySide6.QtWidgets import QFileDialog, QMainWindow, QMessageBox

# noinspection PyUnresolvedReferences
from __feature__ import snake_case, true_property


from azure_config_common import AzureSettingsWidget
from common_qt_functions import (
    CustomConfigFunctions as config_funcs,
    DefaultSettingsManager as settings_manager,
)
from s3_config_common import S3SettingsWidget
from about_page import AboutPage
from utils import set_path_validator
from ui_mount_primary_window import Ui_primaryFUSEwindow

bucket_options = ['s3storage', 'azstorage']
MOUNT_TARGET_COMPONENT = 3
CLOUDFUSE_CLI = 'cloudfuse'
MOUNT_DIR_SUFFIX = ''

if platform == 'win32':
    # on Windows, the cli command ends in '.exe'
    CLOUDFUSE_CLI += '.exe'
    # on Windows, the mount directory must not exist before mounting,
    # so name a non-existent subdirectory of the user-chosen path
    MOUNT_DIR_SUFFIX = 'cloudfuse'

# if cloudfuse is not in the path, look for it in the current directory
if which(CLOUDFUSE_CLI) is None:
    CLOUDFUSE_CLI = './' + CLOUDFUSE_CLI


class FUSEWindow(settings_manager, config_funcs, QMainWindow, Ui_primaryFUSEwindow):
    """
    A class to manage the main Cloudfuse application window.

    Attributes:
        my_window (QSettings): QSettings object for storing window state.
        settings (dict): Configuration settings for Cloudfuse.
    """
    def __init__(self):
        """Initialize the FUSEWindow class."""
        super().__init__()
        self.setup_ui(self)
        self.set_window_title('Cloudfuse')
        self.my_window = QSettings('Cloudfuse', 'Mainwindow')
        self.init_mount_point()
        self.check_config_directory()
        self.textEdit_output.set_read_only(True)
        self.settings = self.all_mount_settings
        self.init_settings_from_config(self.settings)

        set_path_validator(self.lineEdit_mountPoint)

        # Set up the signals for all the interactive entities
        self.button_browse.clicked.connect(self.get_file_dir_input)
        self.button_config.clicked.connect(self.show_settings_widget)
        self.button_mount.clicked.connect(self.mount_bucket)
        self.button_unmount.clicked.connect(self.unmount_bucket)
        self.actionAbout_Qt.triggered.connect(
            self.show_about_qt_page)
        self.actionAbout_CloudFuse.triggered.connect(
            self.show_about_cloudfuse_page)
        self.lineEdit_mountPoint.editingFinished.connect(
            self.update_mount_point_in_settings
        )
        self.dropDown_bucketSelect.currentIndexChanged.connect(
            self.modify_pipeline)

        if platform == 'win32':
            self.lineEdit_mountPoint.set_tool_tip(
                'Designate a new location to mount the bucket, do not create the directory'
            )
            self.button_browse.set_tool_tip(
                "Browse to a new location but don't create a new directory"
            )
        else:
            self.lineEdit_mountPoint.set_tool_tip(
                'Designate a location to mount the bucket - the directory must already exist'
            )
            self.button_browse.set_tool_tip('Browse to a pre-existing directory')

    def check_config_directory(self):
        """Create config directory if it doesn't exist."""
        working_dir = self.get_working_dir()
        if not os.path.isdir(working_dir):
            try:
                os.mkdir(working_dir)
            except OSError as e:
                self.add_output_text(f"Failed to make own path: {str(e)}")

    def init_mount_point(self):
        """Initialize the mount point."""
        try:
            directory = self.my_window.value('mountPoint')
            self.lineEdit_mountPoint.set_text(directory)
        except:
            # Nothing in the settings for mountDir, leave mountPoint blank
            return

    def update_mount_point_in_settings(self):
        """Update the mount point in the settings."""
        try:
            directory = str(self.lineEdit_mountPoint.text())
            self.my_window.set_value('mountPoint', directory)
        except:
            # Couldn't update the settings
            return

    # Define the slots that will be triggered when the signals in Qt are activated

    # There are unique settings per bucket selected for the pipeline,
    # so we must use different widgets to show the different settings
    def show_settings_widget(self):
        """Show the S3 or Azure settings."""
        target_index = self.dropDown_bucketSelect.current_index()
        if bucket_options[target_index] == 's3storage':
            self.set_configs = S3SettingsWidget(self.settings)
        else:
            self.set_configs = AzureSettingsWidget(self.settings)
        self.set_configs.set_window_modality(Qt.ApplicationModal)
        self.set_configs.show()

    def get_file_dir_input(self):
        """Open a file dialog to select a directory."""
        directory = str(QFileDialog.get_existing_directory())
        # get_existing_directory() returns a null string when cancel is selected
        # don't update the lineEdit and settings if cancelled
        if directory != '':
            self.lineEdit_mountPoint.set_text(f'{directory}')
            self.update_mount_point_in_settings()

    def show_about_qt_page(self):
        """Display the about Qt page."""
        QMessageBox.about_qt(self, 'About QT')

    # Display the custom dialog box for the cloudfuse 'about' page.
    def show_about_cloudfuse_page(self):
        """Display the custom dialog box for the cloudfuse 'about' page."""
        command_parts = [CLOUDFUSE_CLI, '--version']
        std_out, _, _, executable_found = self.run_command(command_parts)

        if not executable_found:
            cloudfuse_version = 'Cloudfuse program not present'
        elif std_out != '':
            cloudfuse_version = std_out
        else:
            cloudfuse_version = 'Cloudfuse version not found'

        self.page = AboutPage(cloudfuse_version)
        self.page.show()

    def mount_bucket(self):
        """Mount the selected bucket to the specified directory."""
        # get mount directory
        try:
            directory = str(self.lineEdit_mountPoint.text())
        except ValueError as e:
            self.add_output_text(f"Invalid mount path: {str(e)}")
            return
        directory = os.path.join(directory, MOUNT_DIR_SUFFIX)
        # get config path
        config_path = os.path.join(self.get_working_dir(), 'config.yaml')

        # on Windows, the mount directory should not exist (yet)
        if platform == 'win32':
            if os.path.exists(directory):
                self.add_output_text(
                    f"Directory {directory} already exists! Aborting new mount."
                )
                self.error_msg_box(
                    f"Error: Cloudfuse needs to create the directory {directory}, but it already exists!"
                )
                return

        # do a dry run to validate options and credentials
        command_parts = [
            CLOUDFUSE_CLI,
            'mount',
            directory,
            f"--config-file={config_path}",
            '--dry-run',
        ]
        std_out, std_err, exit_code, executable_found = self.run_command(
            command_parts)
        if not executable_found:
            self.add_output_text('cloudfuse.exe not found! Is it installed?')
            self.error_msg_box(
                'Error running cloudfuse CLI - Please re-install Cloudfuse.'
            )
            return

        if exit_code != 0:
            self.add_output_text(std_err)
            self.error_msg_box('Mount failed: ' + std_err)
            return

        if std_out != '':
            self.add_output_text(std_out)

        # now actually mount
        command_parts = [
            CLOUDFUSE_CLI,
            'mount',
            directory,
            f"--config-file={config_path}",
        ]
        std_out, std_err, exit_code, executable_found = self.run_command(
            command_parts)
        if not executable_found:
            self.add_output_text('cloudfuse.exe not found! Is it installed?')
            self.error_msg_box(
                'Error running cloudfuse CLI - Please re-install Cloudfuse.'
            )
            return

        if exit_code != 0:
            self.add_output_text(f"Error mounting container: {std_err}")
            if 'mount path exists' in std_err:
                self.error_msg_box(
                    'This container is already mounted at this directory.'
                )
            else:
                self.error_msg_box(
                    f"Error mounting container - check the settings and try again\n{std_err}"
                )
            return

        if std_out != '':
            self.add_output_text(std_out)

        # wait for mount, then check that mount succeeded by verifying that the mount directory exists
        self.add_output_text('Verifying mount success...')

        def verify_mount_success():
            if platform == 'win32':
                success = os.path.exists(directory)
            else:
                success = os.path.ismount(directory)
            if not success:
                self.add_output_text(
                    f"Failed to create mount directory {directory}")
                self.error_msg_box('Mount failed. Please check error logs.')
            else:
                self.add_output_text('Successfully mounted container')

        QTimer.singleShot(4000, verify_mount_success)

    def unmount_bucket(self):
        """Unmount the selected bucket from the specified directory."""
        directory = str(self.lineEdit_mountPoint.text())
        # TODO: properly handle unmount. This is relying on the line_edit not being changed by the user.
        directory = os.path.join(directory, MOUNT_DIR_SUFFIX)
        command_parts = [CLOUDFUSE_CLI, 'unmount', directory]
        if platform != 'win32':
            command_parts.append('--lazy')

        _, std_err, exit_code, executable_found = self.run_command(
            command_parts)
        if not executable_found:
            self.add_output_text('cloudfuse.exe not found! Is it installed?')
            self.error_msg_box(
                'Error running cloudfuse CLI - Please re-install Cloudfuse.'
            )
        elif exit_code != 0:
            self.add_output_text(f"Failed to unmount container: {std_err}")
            self.error_msg_box(f"Failed to unmount container: {std_err}")
        else:
            self.add_output_text(f"Successfully unmounted container {std_err}")

    # This function reads in the config file, modifies the components section, then writes the config file back
    def modify_pipeline(self):
        """Modify the pipeline configuration based on the selected bucket."""
        self.add_output_text('Validating configuration...')
        # Update the pipeline/components before mounting the target
        target_bucket = bucket_options[self.dropDown_bucketSelect.current_index(
        )]
        components = self.settings.get('components')
        if components is not None:
            components[MOUNT_TARGET_COMPONENT] = target_bucket
            self.settings['components'] = components
        else:
            working_dir = self.get_working_dir()
            self.error_msg_box(
                f"The components is missing in {working_dir}/config.yaml. Consider going through the settings to create one.",
                'Components in config missing',
            )
            return
        self.write_config_file(self.settings)

    def run_command(self, command_parts: list) -> tuple:
        """
        Run a command as a subprocess and return a tuple containing:
        (std_out, std_err, exit_code, executable_found).

        Args:
            command_parts (list): List of command parts to run.

        Returns:
            tuple: (std_out, std_err, exit_code, executable_found)
        """
        if len(command_parts) < 1:
            # (std_out, std_err, exit_code, executable_found)
            return ('', '', -1, False)
        # run command
        try:
            process = subprocess.run(
                command_parts,
                capture_output=True,
                creationflags=(
                    subprocess.CREATE_NO_WINDOW if hasattr(
                        subprocess, 'CREATE_NO_WINDOW') else 0
                ),
                check=False,
            )
            std_out = process.stdout.decode().strip()
            std_err = process.stderr.decode().strip()
            exit_code = process.returncode
            return (std_out, std_err, exit_code, True)
        except FileNotFoundError:
            return ('', '', -1, False)
        except PermissionError:
            return ('', '', -1, False)

    def add_output_text(self, text: str):
        """
        Add text to the output text edit widget.

        Args:
            text (str): The text to add.
        """
        self.textEdit_output.append(text)
        self.textEdit_output.move_cursor(QTextCursor.End)

    def error_msg_box(self, message: str, title='Error'):
        """
        Display an error message box with the given message and title.

        Args:
            message_string (str): The message to display.
            title_string (str): The title of the message box.
        """
        msg = QMessageBox()
        msg.set_icon(QMessageBox.Critical)
        msg.set_window_title(title)
        msg.set_text(message)
        msg.exec()
