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

# System imports
import yaml
import os
from sys import platform

# Import QT libraries
from PySide6 import QtWidgets
from PySide6.QtWidgets import QWidget
from PySide6.QtCore import Qt, QSettings
from PySide6.QtGui import QScreen

libfusePermissions = [0o777,0o666,0o644,0o444]

class defaultSettingsManager():
    def __init__(self):
        super().__init__()
        self.allMountSettings = {}
        self.setAllDefaultSettings(self.allMountSettings)

    def setAllDefaultSettings(self, allMountSettings):
        self.setS3Settings(allMountSettings)
        self.setAzureSettings(allMountSettings)
        self.setComponentSettings(allMountSettings)

    def setS3Settings(self, allMountSettings):
        # REFER TO ~/setup/baseConfig.yaml for explanations of what these settings are
        allMountSettings['s3storage'] = {
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
            'usePathStyle': False
            }

    def setAzureSettings(self,allMountSettings):
        # REFER TO ~/setup/baseConfig.yaml for explanations of what these settings are
        allMountSettings['azstorage'] = {
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
            'oauth-token-path': '', # not exposed
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
            'honour-acl': False
            }

    def setComponentSettings(self, allMountSettings):
        # REFER TO ~/setup/baseConfig.yaml for explanations of what these settings are

        allMountSettings['foreground'] = False
        # Common
        allMountSettings['allow-other'] = False
        allMountSettings['read-only'] = False
        allMountSettings['nonempty'] = False
        allMountSettings['restricted-characters-windows'] = False
        # Profiler
        allMountSettings['dynamic-profile'] = False
        allMountSettings['profiler-port'] = 6060
        allMountSettings['profiler-ip'] = 'localhost'
        # Pipeline components
        allMountSettings['components'] = ['libfuse','file_cache','attr_cache','s3storage']
        # Sub-sections
        allMountSettings['libfuse'] = {
            'default-permission' : 0o777,
            'attribute-expiration-sec': 120,
            'entry-expiration-sec' : 120,
            'negative-entry-expiration-sec' : 120,
            'fuse-trace' : False,
            'extension' : '',
            'disable-writeback-cache' : False,
            'ignore-open-flags' : True,
            'max-fuse-threads': 128,
            'direct-io': False, # not exposed
            'network-share': False
            }

        allMountSettings['stream'] = {
            'block-size-mb': 0,
            'max-buffers': 0,
            'buffer-size-mb': 0,
            'file-caching': False # false = handle level caching ON
            }

        # the block cache component and its settings are not exposed in the GUI
        allMountSettings['block_cache'] = {
            'block-size-mb': 16,
            'mem-size-mb': 4192,
            'path': '',
            'disk-size-mb': 4192,
            'disk-timeout-sec': 120,
            'prefetch': 11,
            'parallelism': 128,
            'prefetch-on-open': False
            }

        allMountSettings['file_cache'] = {
            'path': '',
            'timeout-sec' : 216000,
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
            'hard-limit': False # not exposed
            }

        allMountSettings['attr_cache'] = {
            'timeout-sec': 120,
            'no-cache-on-list': False,
            'enable-symlinks': False,
            # the following attr_cache settings are not exposed in the GUI
            'max-files': 5000000,
            'no-cache-dirs': False
            }

        allMountSettings['loopbackfs'] = {
            'path': ''
            }

        allMountSettings['mountall'] = {
            'container-allowlist': [],
            'container-denylist': []
            }

        allMountSettings['health_monitor'] = {
            'enable-monitoring': False,
            'stats-poll-interval-sec': 10,
            'process-monitor-interval-sec': 30,
            'output-path':'',
            'monitor-disable-list': []
            }

        allMountSettings['logging'] = {
            'type' : 'base',
            'level' : 'log_warning',
            'file-path' : '$HOME/.cloudfuse/cloudfuse.log',
            'max-file-size-mb' : 512,
            'file-count' : 10 ,
            'track-time' : False
            }

class customConfigFunctions():
    def __init__(self):
        super().__init__()

    # defaultSettingsManager has set the settings to all default, now the code needs to pull in
    #   all the changes from the config file the user provides. This may not include all the
    #   settings defined in defaultSettingManager.
    def initSettingsFromConfig(self, settings):
        dictForConfigs = self.getConfigs(settings)
        for option in dictForConfigs:
            # check default settings to enforce YAML schema
            invalidOption = not (option in settings)
            invalidType = type(settings.get(option, None)) != type(dictForConfigs[option])
            if invalidOption or invalidType:
                print(f"WARNING: Ignoring invalid config option: {option} (type mismatch)")
                continue
            if type(dictForConfigs[option]) == dict:
                tempDict = settings[option]
                for suboption in dictForConfigs[option]:
                    tempDict[suboption] = dictForConfigs[option][suboption]
                settings[option]=tempDict
            else:
                settings[option] = dictForConfigs[option]

    def getConfigs(self,settings,useDefault=False):
        workingDir = self.getWorkingDir()
        if useDefault:
            # Use programmed defaults
            defaultSettingsManager.setAllDefaultSettings(self,settings)
            configs = settings
        else:
            try:
                with open(workingDir+'/config.yaml', 'r') as file:
                    configs = yaml.safe_load(file)
                    if configs is None:
                       # The configs file exists, but is empty, use default settings
                       configs = self.getConfigs(settings,True)
            except:
                # Could not open or config file does not exist, use default settings
                configs = self.getConfigs(settings,True)
        return configs


    def getWorkingDir(self):
        if platform == 'win32':
            defaultFuseDir = 'Cloudfuse'
            userDir = os.getenv('APPDATA')
        else:
            defaultFuseDir = '.cloudfuse'
            userDir = os.getenv('HOME')
        workingDir = os.path.join(userDir, defaultFuseDir)
        return workingDir

    def writeConfigFile(self, settings):
        workingDir = self.getWorkingDir()
        try:
            with open(workingDir+'/config.yaml','w') as file:
                yaml.safe_dump(settings,file)
                return True
        except:
            msg = QtWidgets.QMessageBox()
            msg.setWindowTitle('Write Failed')
            msg.setInformativeText('Writing the config file failed. Check file permissions and try again.')
            msg.exec()
            return False

class widgetCustomFunctions(customConfigFunctions,QWidget):
    def __init__(self):
        super().__init__()

    def exitWindow(self):
        self.saveButtonClicked = True
        self.close()

    def exitWindowCleanup(self):
    # Save this specific window's size and position
        self.myWindow.setValue('window size', self.size())
        self.myWindow.setValue('window position', self.pos())

    def popupDoubleCheckReset(self):
        checkMsg = QtWidgets.QMessageBox()
        checkMsg.setWindowTitle('Are you sure?')
        checkMsg.setInformativeText('ResetDefault settings will reset all settings for this target.')
        checkMsg.setStandardButtons(QtWidgets.QMessageBox.Cancel | QtWidgets.QMessageBox.Yes)
        checkMsg.setDefaultButton(QtWidgets.QMessageBox.Cancel)
        choice = checkMsg.exec()
        return choice

    # Overrides the closeEvent function from parent class to enable this custom behavior
    # TODO: Nice to have - keep track of changes to user makes and only trigger the 'are you sure?' message
    #   when changes have been made
    def closeEvent(self, event):
        msg = QtWidgets.QMessageBox()
        msg.setWindowTitle('Are you sure?')
        msg.setInformativeText('Do you want to save you changes?')
        msg.setText('The settings have been modified.')
        msg.setStandardButtons(QtWidgets.QMessageBox.Discard | QtWidgets.QMessageBox.Cancel | QtWidgets.QMessageBox.Save)
        msg.setDefaultButton(QtWidgets.QMessageBox.Cancel)

        if self.saveButtonClicked == True:
            # Insert all settings to yaml file
            self.exitWindowCleanup()
            self.updateSettingsFromUIChoices()
            if self.writeConfigFile(self.settings):
                event.accept()
            else:
                event.ignore()
        else:
            ret = msg.exec()
            if ret == QtWidgets.QMessageBox.Discard:
                self.exitWindowCleanup()
                event.accept()
            elif ret == QtWidgets.QMessageBox.Cancel:
                event.ignore()
            elif ret == QtWidgets.QMessageBox.Save:
                # Insert all settings to yaml file
                self.exitWindowCleanup()
                self.updateSettingsFromUIChoices()
                if self.writeConfigFile(self.settings):
                    event.accept()
                else:
                    event.ignore()

    def updateSettingsFromUIChoices(self):
        # Each individual widget will need to override this function
        pass

    def initWindowSizePos(self):
        try:
            self.resize(self.myWindow.value('window size'))
            self.move(self.myWindow.value('window position'))
        except:
            desktopCenter = QScreen.availableGeometry(QtWidgets.QApplication.primaryScreen()).center()
            myWindowGeometry = self.frameGeometry()
            myWindowGeometry.moveCenter(desktopCenter)
            self.move(myWindowGeometry.topLeft())

    # Check for a true/false setting and set the checkbox state as appropriate.
    #   Note, Checked/UnChecked are NOT True/False data types, hence the need to check what the values are.
    #   The default values for True/False settings are False, which is why Unchecked is the default state if the value doesn't equate to True.
    #   Explicitly check for True for clarity
    def setCheckboxFromSetting(self, checkbox, settingName):
        if settingName == True:
            checkbox.setCheckState(Qt.Checked)
        else:
            checkbox.setCheckState(Qt.Unchecked)

    def updateMultiUser(self):
        self.settings['allow-other'] = self.checkBox_multiUser.isChecked()

    def updateNonEmtpyDir(self):
        self.settings['nonempty'] = self.checkBox_nonEmptyDir.isChecked()

    def updateDaemonForeground(self):
        self.settings['foreground'] = self.checkBox_daemonForeground.isChecked()

    def updateReadOnly(self):
        self.settings['read-only'] = self.checkBox_readOnly.isChecked()

    # Update Libfuse re-writes everything in the Libfuse because of how setting.setValue works -
    #   it will not append, so the code makes a copy of the dictionary and updates the sub-keys.
    #   When the user updates the sub-option through the GUI, it will trigger Libfuse to update;
    #   it's written this way to save on lines of code.
    def updateLibfuse(self):
        libfuse = self.settings['libfuse']
        libfuse['default-permission'] = libfusePermissions[self.dropDown_libfuse_permissions.currentIndex()]
        libfuse['ignore-open-flags'] = self.checkBox_libfuse_ignoreAppend.isChecked()
        libfuse['attribute-expiration-sec'] = self.spinBox_libfuse_attExp.value()
        libfuse['entry-expiration-sec'] = self.spinBox_libfuse_entExp.value()
        libfuse['negative-entry-expiration-sec'] = self.spinBox_libfuse_negEntryExp.value()
        self.settings['libfuse'] = libfuse

    def updateOptionalLibfuse(self):
        libfuse = self.settings['libfuse']
        libfuse['disable-writeback-cache'] = self.checkBox_libfuse_disableWriteback.isChecked()
        libfuse['network-share'] = self.checkBox_libfuse_networkshare.isChecked()
        libfuse['max-fuse-threads'] = self.spinBox_libfuse_maxFuseThreads.value()
        self.settings['libfuse'] = libfuse

    # Update stream re-writes everything in the stream dictionary for the same reason update libfuse does.
    def updateStream(self):
        stream = self.settings['stream']
        stream['file-caching'] = self.checkBox_streaming_fileCachingLevel.isChecked()
        stream['block-size-mb'] = self.spinBox_streaming_blockSize.value()
        stream['buffer-size-mb'] = self.spinBox_streaming_buffSize.value()
        stream['max-buffers'] = self.spinBox_streaming_maxBuff.value()
        self.settings['stream'] = stream

    def updateFileCachePath(self):
        filePath = self.settings['file_cache']
        filePath['path'] = self.lineEdit_fileCache_path.text()
        self.settings['file_cache'] = filePath

    def updateOptionalFileCache(self):
        fileCache = self.settings['file_cache']
        fileCache['allow-non-empty-temp'] = self.checkBox_fileCache_allowNonEmptyTmp.isChecked()
        fileCache['policy-trace'] = self.checkBox_fileCache_policyLogs.isChecked()
        fileCache['create-empty-file'] = self.checkBox_fileCache_createEmptyFile.isChecked()
        fileCache['cleanup-on-start'] = self.checkBox_fileCache_cleanupStart.isChecked()
        fileCache['offload-io'] = self.checkBox_fileCache_offloadIO.isChecked()
        fileCache['sync-to-flush'] = self.checkBox_fileCache_syncToFlush.isChecked()

        fileCache['timeout-sec'] = self.spinBox_fileCache_evictionTimeout.value()
        fileCache['max-eviction'] = self.spinBox_fileCache_maxEviction.value()
        fileCache['max-size-mb'] = self.spinBox_fileCache_maxCacheSize.value()
        fileCache['high-threshold'] = self.spinBox_fileCache_evictMaxThresh.value()
        fileCache['low-threshold'] = self.spinBox_fileCache_evictMinThresh.value()
        fileCache['refresh-sec'] = self.spinBox_fileCache_refreshSec.value()

        self.settings['file_cache'] = fileCache
