"""
Defines classes for managing default settings and custom configuration functions.
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
from sys import platform

# System imports
import yaml
# Import QT libraries
from PySide6.QtGui import QCloseEvent, QScreen
from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget, QMessageBox, QApplication, QCheckBox

file_cache_eviction_choices = ['lru', 'lfu']
libfusePermissions = [0o777, 0o666, 0o644, 0o444]


class DefaultSettingsManager:
    """
    A class to manage default settings for various storage configurations.

    Attributes:
        all_mount_settings (dict): Dictionary to store all mount settings.
    """
    def __init__(self):
        """
        Initialize the DefaultSettingsManager with default settings.
        """
        super().__init__()
        self.all_mount_settings = {}
        self.set_all_default_settings(self.all_mount_settings)

    def set_all_default_settings(self, all_mount_settings: dict):
        """
        Set all default settings for S3, Azure, and components.

        Args:
            all_mount_settings (dict): Dictionary to store all mount settings.
        """
        self.set_s3_settings(all_mount_settings)
        self.set_azure_settings(all_mount_settings)
        self.set_component_settings(all_mount_settings)

    def set_s3_settings(self, all_mount_settings: dict):
        """
        Set default S3 storage settings.

        Args:
            all_mount_settings (dict): Dictionary to store all mount settings.
        """
        # REFER TO ~/setup/baseConfig.yaml for explanations of what these settings are
        all_mount_settings['s3storage'] = {
            'bucket-name': '',
            'key-id': '',
            'secret-key': '',
            'region': '',
            'endpoint': '',
            'subdirectory': '',
            # the following S3 options are not exposed in the GUI
            # TODO: which options should be exposed?
            'profile': '',
            'part-size-mb': 8,
            'upload-cutoff-mb': 100,
            'concurrency': 5,
            'disable-concurrent-download': False,
            'enable-checksum': False,
            'checksum-algorithm': 'SHA1',
            'usePathStyle': False,
        }

    def set_azure_settings(self, all_mount_settings: dict):
        """
        Set default Azure storage settings.

        Args:
            all_mount_settings (dict): Dictionary to store all mount settings.
        """
        # REFER TO ~/setup/baseConfig.yaml for explanations of what these settings are
        all_mount_settings['azstorage'] = {
            'type': 'block',
            'account-name': '',
            'container': '',
            'endpoint': '',
            'mode': 'key',
            'account-key': '',
            'sas': '',
            'appid': '',
            'resid': '',
            'objid': '',
            'tenantid': '',
            'clientid': '',
            'clientsecret': '',
            'oauth-token-path': '',  # not exposed
            'use-http': False,
            'aadendpoint': '',
            'subdirectory': '',
            'block-size-mb': 16,
            'max-concurrency': 32,
            'tier': 'none',
            'block-list-on-mount-sec': 0,
            'max-retries': 5,
            'max-retry-timeout-sec': 900,
            'retry-backoff-sec': 4,
            'max-retry-delay-sec': 60,
            'http-proxy': '',
            'https-proxy': '',
            'sdk-trace': False,
            'fail-unsupported-op': False,
            'auth-resource': '',
            'update-md5': False,
            'validate-md5': False,
            'virtual-directory': True,
            'disable-compression': False,
            # the following Azure options are not exposed in the GUI
            'max-results-for-list': 2,
            'telemetry': '',
            'honour-acl': False,
        }

    def set_component_settings(self, all_mount_settings: dict):
        """
        Set default component settings.

        Args:
            all_mount_settings (dict): Dictionary to store all mount settings.
        """
        # REFER TO ~/setup/baseConfig.yaml for explanations of what these settings are

        all_mount_settings['foreground'] = False
        # Common
        all_mount_settings['allow-other'] = False
        all_mount_settings['read-only'] = False
        all_mount_settings['nonempty'] = False
        all_mount_settings['restricted-characters-windows'] = False
        # Profiler
        all_mount_settings['dynamic-profile'] = False
        all_mount_settings['profiler-port'] = 6060
        all_mount_settings['profiler-ip'] = 'localhost'
        # Pipeline components
        all_mount_settings['components'] = [
            'libfuse',
            'file_cache',
            'attr_cache',
            's3storage',
        ]
        # Sub-sections
        all_mount_settings['libfuse'] = {
            'default-permission': 0o777,
            'attribute-expiration-sec': 120,
            'entry-expiration-sec': 120,
            'negative-entry-expiration-sec': 120,
            'fuse-trace': False,
            'extension': '',
            'disable-writeback-cache': False,
            'ignore-open-flags': True,
            'max-fuse-threads': 128,
            'direct-io': False,  # not exposed
            'network-share': False,
        }

        all_mount_settings['stream'] = {
            'block-size-mb': 0,
            'max-buffers': 0,
            'buffer-size-mb': 0,
            'file-caching': False,  # false = handle level caching ON
        }

        # the block cache component and its settings are not exposed in the GUI
        all_mount_settings['block_cache'] = {
            'block-size-mb': 16,
            'mem-size-mb': 4192,
            'path': '',
            'disk-size-mb': 4192,
            'disk-timeout-sec': 120,
            'prefetch': 11,
            'parallelism': 128,
            'prefetch-on-open': False,
        }

        all_mount_settings['file_cache'] = {
            'path': '',
            'policy': 'lru',
            'timeout-sec': 64000000,
            'max-eviction': 5000,
            'max-size-mb': 0,
            'high-threshold': 80,
            'low-threshold': 60,
            'create-empty-file': False,
            'allow-non-empty-temp': True,
            'cleanup-on-start': False,
            'policy-trace': False,
            'offload-io': False,
            'sync-to-flush': True,
            'refresh-sec': 60,
            'ignore-sync': True,
            'hard-limit': False,  # not exposed
        }

        all_mount_settings['attr_cache'] = {
            'timeout-sec': 120,
            'no-cache-on-list': False,
            'enable-symlinks': False,
            # the following attr_cache settings are not exposed in the GUI
            'max-files': 5000000,
            'no-cache-dirs': False,
        }

        all_mount_settings['loopbackfs'] = {'path': ''}

        all_mount_settings['mountall'] = {
            'container-allowlist': [],
            'container-denylist': [],
        }

        all_mount_settings['health_monitor'] = {
            'enable-monitoring': False,
            'stats-poll-interval-sec': 10,
            'process-monitor-interval-sec': 30,
            'output-path': '',
            'monitor-disable-list': [],
        }

        all_mount_settings['logging'] = {
            'type': 'syslog',
            'level': 'log_err',
            'file-path': '$HOME/.cloudfuse/cloudfuse.log',
            'max-file-size-mb': 512,
            'file-count': 10,
            'track-time': False,
        }


class CustomConfigFunctions:
    """
    A class to manage custom configuration functions.

    Methods:
        init_settings_from_config(settings): Initialize settings from a configuration file.
        get_configs(settings, use_default=False): Get configuration settings.
        get_working_dir(): Get the working directory path.
        write_config_file(settings): Write the configuration settings to a file.
    """
    def __init__(self):
        """
        Initialize the CustomConfigFunctions class.
        """
        super().__init__()

    # defaultSettingsManager has set the settings to all default, now the code needs to pull in
    #   all the changes from the config file the user provides. This may not include all the
    #   settings defined in defaultSettingManager.
    def init_settings_from_config(self, settings: dict):
        """
        Initialize settings from a configuration file.

        Args:
            settings (dict): Dictionary to store all settings.
        """
        dict_for_configs = self.get_configs(settings)
        for option in dict_for_configs:
            # check default settings to enforce YAML schema
            invalid_option = option not in settings
            invalid_type = type(settings.get(option, None)) != type(
                dict_for_configs[option]
            )
            if invalid_option or invalid_type:
                print(
                    f"WARNING: Ignoring invalid config option: {option} (type mismatch)"
                )
                continue
            if type(dict_for_configs[option]) == dict:
                temp_dict = settings[option]
                for suboption in dict_for_configs[option]:
                    temp_dict[suboption] = dict_for_configs[option][suboption]
                settings[option] = temp_dict
            else:
                settings[option] = dict_for_configs[option]

    def get_configs(self, settings: dict, use_default=False) -> dict:
        """
        Get configuration settings.

        Args:
            settings (dict): Dictionary to store all settings.
            use_default (bool): Flag to indicate whether to use default settings.

        Returns:
            dict: Configuration settings.
        """
        working_dir = self.get_working_dir()
        if use_default:
            # Use programmed defaults
            DefaultSettingsManager.set_all_default_settings(self, settings)
            configs = settings
        else:
            try:
                with open(working_dir + '/config.yaml', 'r', encoding='utf-8') as file:
                    configs = yaml.safe_load(file)
                    if configs is None:
                        # The configs file exists, but is empty, use default settings
                        configs = self.get_configs(settings, True)
            except:
                # Could not open or config file does not exist, use default settings
                configs = self.get_configs(settings, True)
        return configs

    def get_working_dir(self) -> str:
        """
        Get the working directory path.

        Returns:
            str: The working directory path.
        """
        if platform == 'win32':
            default_fuse_dir = 'Cloudfuse'
            user_dir = os.getenv('APPDATA')
        else:
            default_fuse_dir = '.cloudfuse'
            user_dir = os.getenv('HOME')
        working_dir = os.path.join(user_dir, default_fuse_dir)
        return working_dir

    def write_config_file(self, settings: dict) -> bool:
        """
        Write the configuration settings to a file.

        Args:
            settings (dict): Dictionary to store all settings.

        Returns:
            bool: True if the file was written successfully, False otherwise.
        """
        working_dir = self.get_working_dir()
        try:
            with open(working_dir + '/config.yaml', 'w') as file:
                yaml.safe_dump(settings, file)
                return True
        except:
            msg = QMessageBox()
            msg.setWindowTitle('Write Failed')
            msg.setInformativeText(
                'Writing the config file failed. Check file permissions and try again.'
            )
            msg.exec()
            return False


class WidgetCustomFunctions(CustomConfigFunctions, QWidget):
    """
    A class to manage custom widget functions.

    Attributes:
        save_button_clicked (bool): Flag to indicate if the save button was clicked.
    """
    def __init__(self):
        """
        Initialize the WidgetCustomFunctions class.
        """
        super().__init__()
        self.save_button_clicked = False

    def exit_window(self):
        """
        Exit the window and set the save button clicked flag.
        """
        self.save_button_clicked = True
        self.close()

    def exit_window_cleanup(self):
        """
        Save the window size and position before exiting.
        """
        # Save this specific window's size and position
        self.myWindow.setValue('window size', self.size())
        self.myWindow.setValue('window position', self.pos())

    def popup_double_check_reset(self) -> int:
        """
        Show a confirmation dialog for resetting settings.

        Returns:
            int: The user's choice from the dialog.
        """
        check_msg = QMessageBox()
        check_msg.setWindowTitle('Are you sure?')
        check_msg.setInformativeText(
            'ResetDefault settings will reset all settings for this target.'
        )
        check_msg.setStandardButtons(
            QMessageBox.Cancel | QMessageBox.Yes
        )
        check_msg.setDefaultButton(QMessageBox.Cancel)
        choice = check_msg.exec()
        return choice

    # Overrides the closeEvent function from parent class to enable this custom behavior
    # TODO: Nice to have - keep track of changes to user makes and only trigger the 'are you sure?' message
    #   when changes have been made
    def closeEvent(self, event: QCloseEvent):
        """
        Handle the close event with a confirmation dialog.

        Args:
            event (QCloseEvent): The close event.
        """
        msg = QMessageBox()
        msg.setWindowTitle('Are you sure?')
        msg.setInformativeText('Do you want to save you changes?')
        msg.setText('The settings have been modified.')
        msg.setStandardButtons(
            QMessageBox.Discard
            | QMessageBox.Cancel
            | QMessageBox.Save
        )
        msg.setDefaultButton(QMessageBox.Cancel)

        if self.save_button_clicked:
            # Insert all settings to yaml file
            self.exit_window_cleanup()
            self.update_settings_from_ui_choices()
            if self.write_config_file(self.settings):
                event.accept()
            else:
                event.ignore()
        else:
            ret = msg.exec()
            if ret == QMessageBox.Discard:
                self.exit_window_cleanup()
                event.accept()
            elif ret == QMessageBox.Cancel:
                event.ignore()
            elif ret == QMessageBox.Save:
                # Insert all settings to yaml file
                self.exit_window_cleanup()
                self.update_settings_from_ui_choices()
                if self.write_config_file(self.settings):
                    event.accept()
                else:
                    event.ignore()

    def update_settings_from_ui_choices(self):
        """
        Update all settings from the UI choices.
        """
        # Each individual widget will need to override this function
        pass

    def init_window_size_pos(self):
        """
        Initialize the window size and position.
        """
        try:
            self.resize(self.myWindow.value('window size'))
            self.move(self.myWindow.value('window position'))
        except:
            desktop_center = QScreen.availableGeometry(
                QApplication.primaryScreen()
            ).center()
            my_window_geometry = self.frameGeometry()
            my_window_geometry.moveCenter(desktop_center)
            self.move(my_window_geometry.topLeft())

    # Check for a true/false setting and set the checkbox state as appropriate.
    #   Note, Checked/UnChecked are NOT True/False data types, hence the need to check what the values are.
    #   The default values for True/False settings are False, which is why Unchecked is the default state if the value doesn't equate to True.
    #   Explicitly check for True for clarity
    def set_checkbox_from_setting(self, checkbox: QCheckBox, setting_name: bool):
        """
        Set the checkbox state based on the setting value.

        Args:
            checkbox (QCheckBox): The checkbox to set.
            setting_name (bool): The setting value.
        """
        if setting_name:
            checkbox.setCheckState(Qt.Checked)
        else:
            checkbox.setCheckState(Qt.Unchecked)

    def update_multi_user(self):
        """
        Update the multi-user setting.
        """
        self.settings['allow-other'] = self.checkBox_multiUser.isChecked()

    def update_non_emtpy_dir(self):
        """
        Update the non-empty directory setting.
        """
        self.settings['nonempty'] = self.checkBox_nonEmptyDir.isChecked()

    def update_daemon_foreground(self):
        """
        Update the daemon foreground setting.
        """
        self.settings['foreground'] = self.checkBox_daemonForeground.isChecked()

    def update_read_only(self):
        """
        Update the read-only setting.
        """
        self.settings['read-only'] = self.checkBox_readOnly.isChecked()

    # Update Libfuse re-writes everything in the Libfuse because of how setting.setValue works -
    # it will not append, so the code makes a copy of the dictionary and updates the sub-keys.
    # When the user updates the sub-option through the GUI, it will trigger Libfuse to update;
    # it's written this way to save on lines of code.
    def update_libfuse(self):
        """
        Update the libfuse settings from the UI choices.
        """
        libfuse = self.settings['libfuse']
        libfuse['default-permission'] = libfusePermissions[
            self.dropDown_libfuse_permissions.currentIndex()
        ]
        libfuse['ignore-open-flags'] = self.checkBox_libfuse_ignoreAppend.isChecked()
        libfuse['attribute-expiration-sec'] = self.spinBox_libfuse_attExp.value()
        libfuse['entry-expiration-sec'] = self.spinBox_libfuse_entExp.value()
        libfuse['negative-entry-expiration-sec'] = (
            self.spinBox_libfuse_negEntryExp.value()
        )
        self.settings['libfuse'] = libfuse

    def update_optional_libfuse(self):
        """
        Update the optional libfuse settings from the UI choices.
        """
        libfuse = self.settings['libfuse']
        libfuse['disable-writeback-cache'] = (
            self.checkBox_libfuse_disableWriteback.isChecked()
        )
        libfuse['network-share'] = self.checkBox_libfuse_networkshare.isChecked()
        libfuse['max-fuse-threads'] = self.spinBox_libfuse_maxFuseThreads.value()
        self.settings['libfuse'] = libfuse

    # Update stream re-writes everything in the stream dictionary for the same reason update libfuse does.
    def update_stream(self):
        """
        Update the stream settings from the UI choices.
        """
        stream = self.settings['stream']
        stream['file-caching'] = self.checkBox_streaming_fileCachingLevel.isChecked()
        stream['block-size-mb'] = self.spinBox_streaming_blockSize.value()
        stream['buffer-size-mb'] = self.spinBox_streaming_buffSize.value()
        stream['max-buffers'] = self.spinBox_streaming_maxBuff.value()
        self.settings['stream'] = stream

    def update_file_cache_path(self):
        """
        Update the file cache path from the UI values.
        """
        file_path = self.settings['file_cache']
        file_path['path'] = self.lineEdit_fileCache_path.text()
        self.settings['file_cache'] = file_path

    def update_optional_file_cache(self):
        """
        Update the optional file cache settings from the UI values.
        """
        file_cache = self.settings['file_cache']
        file_cache['allow-non-empty-temp'] = (
            self.checkBox_fileCache_allowNonEmptyTmp.isChecked()
        )
        file_cache['policy-trace'] = self.checkBox_fileCache_policyLogs.isChecked()
        file_cache['create-empty-file'] = (
            self.checkBox_fileCache_createEmptyFile.isChecked()
        )
        file_cache['cleanup-on-start'] = self.checkBox_fileCache_cleanupStart.isChecked()
        file_cache['offload-io'] = self.checkBox_fileCache_offloadIO.isChecked()
        file_cache['sync-to-flush'] = self.checkBox_fileCache_syncToFlush.isChecked()

        file_cache['timeout-sec'] = self.spinBox_fileCache_evictionTimeout.value()
        file_cache['max-eviction'] = self.spinBox_fileCache_maxEviction.value()
        file_cache['max-size-mb'] = self.spinBox_fileCache_maxCacheSize.value()
        file_cache['high-threshold'] = self.spinBox_fileCache_evictMaxThresh.value()
        file_cache['low-threshold'] = self.spinBox_fileCache_evictMinThresh.value()
        file_cache['refresh-sec'] = self.spinBox_fileCache_refreshSec.value()

        file_cache['policy'] = file_cache_eviction_choices[
            self.dropDown_fileCache_evictionPolicy.currentIndex()
        ]
        self.settings['file_cache'] = file_cache
