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
import subprocess
from sys import platform
import os
from shutil import which
import ctypes
import string
import datetime

# Import QT libraries
from PySide6.QtCore import Qt, QSettings
from PySide6 import QtWidgets, QtGui, QtCore
from PySide6.QtWidgets import QMainWindow

# Import the custom class created with QtDesigner
from ui_mountPrimaryWindow import Ui_primaryFUSEwindow
from s3_config_common import s3SettingsWidget
from azure_config_common import azureSettingsWidget
from aboutPage import aboutPage
from common_qt_functions import defaultSettingsManager as settingsManager, customConfigFunctions as configFuncs

bucketOptions = ['s3storage', 'azstorage']
mountTargetComponent = 3
cloudfuseCli = 'cloudfuse'
mountDirSuffix = ''
if platform == 'win32':
    # on Windows, the cli command ends in '.exe'
    cloudfuseCli += '.exe'
    # on Windows, the mound directory must not exist before mounting,
    # so name a non-existent subdirectory of the user-chosen path
    mountDirSuffix = 'cloudfuse'
#  if cloudfuse is not in the path, look for it in the current directory
if which(cloudfuseCli) is None:
    cloudfuseCli = './' + cloudfuseCli

class FUSEWindow(settingsManager,configFuncs, QMainWindow, Ui_primaryFUSEwindow):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle('Cloudfuse')
        self.myWindow = QSettings('Cloudfuse', 'Mainwindow')
        self.initMountPoint()
        self.checkConfigDirectory()
        self.textEdit_output.setReadOnly(True)
        self.settings = self.allMountSettings
        self.initSettingsFromConfig(self.settings)

        if platform == 'win32':
            # Windows directory and filename conventions:
            #   https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#file-and-directory-names
            # Disallow the following [<,>,.,",|,?,*] - note, we still need directory characters to declare a path
            self.lineEdit_mountPoint.setValidator(QtGui.QRegularExpressionValidator(r'^[^<>."|?\0*]*$',self))
            self.button_browse.setText('Drive Letter')
            self.button_browse.setToolTip('Select an unused drive letter for mounting')
            self.button_browse.clicked.connect(self.chooseDriveLetter)
        else:
            # Allow anything BUT Nul
            # Note: Different versions of Python don't like the embedded null character, send in the raw string instead
            self.lineEdit_mountPoint.setValidator(QtGui.QRegularExpressionValidator(r'^[^\0]*$',self))
            self.button_browse.setText('Browse')
            self.button_browse.setToolTip('Browse to a pre-existing directory to mount')
            self.button_browse.clicked.connect(self.getFileDirInput)

            # The remount option is not supported on Linux
            self.checkBox_remount.hide()

        # Set up the signals for all the interactive entities
        self.button_config.clicked.connect(self.showSettingsWidget)
        self.button_mount.clicked.connect(self.mountBucket)
        self.button_unmount.clicked.connect(self.unmountBucket)
        self.actionAbout_Qt.triggered.connect(self.showAboutQtPage)
        self.actionAbout_CloudFuse.triggered.connect(self.showAboutCloudFusePage)
        self.lineEdit_mountPoint.editingFinished.connect(self.updateMountPointInSettings)
        self.dropDown_bucketSelect.currentIndexChanged.connect(self.modifyPipeline)
        if platform == 'win32':
            self.lineEdit_mountPoint.setToolTip('Designate a drive letter to mount to')
        else:
            self.lineEdit_mountPoint.setToolTip('Designate a location to mount the bucket - the directory must already exist')

    def checkConfigDirectory(self):
        workingDir = self.getWorkingDir()
        if not os.path.isdir(workingDir):
            try:
                os.mkdir(workingDir)
            except OSError as e:
                self.addOutputText(f"Failed to make own path: {str(e)}")

    def initMountPoint(self):
        try:
            directory = self.myWindow.value('mountPoint')
            self.lineEdit_mountPoint.setText(directory)
        except:
            # Nothing in the settings for mountDir, leave mountPoint blank
            return

    def updateMountPointInSettings(self):
        try:
            directory = str(self.lineEdit_mountPoint.text())
            self.myWindow.setValue('mountPoint',directory)
        except:
            # Couldn't update the settings
            return

    # Define the slots that will be triggered when the signals in Qt are activated

    # There are unique settings per bucket selected for the pipeline,
    #   so we must use different widgets to show the different settings
    def showSettingsWidget(self):
        targetIndex = self.dropDown_bucketSelect.currentIndex()
        if bucketOptions[targetIndex] == 's3storage':
            self.setConfigs = s3SettingsWidget(self.settings)
        else:
            self.setConfigs = azureSettingsWidget(self.settings)
        self.setConfigs.setWindowModality(Qt.ApplicationModal)
        self.setConfigs.show()

    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        # getExistingDirectory() returns a null string when cancel is selected
        #   don't update the lineEdit and settings if cancelled
        if directory != '':
            self.lineEdit_mountPoint.setText('{}'.format(directory))
            self.updateMountPointInSettings()


    # Display the pre-baked about QT messagebox
    def showAboutQtPage(self):
        QtWidgets.QMessageBox.aboutQt(self, 'About QT')

    # Display the custom dialog box for the cloudfuse 'about' page.
    def showAboutCloudFusePage(self):
        commandParts = [cloudfuseCli, '--version']
        (stdOut, stdErr, exitCode, executableFound) = self.runCommand(commandParts)

        if not executableFound:
            cloudfuseVersion = 'Cloudfuse program not present'
        elif stdOut != '':
            cloudfuseVersion = stdOut
        else:
            cloudfuseVersion = 'Cloudfuse version not found'

        self.page = aboutPage(cloudfuseVersion)
        self.page.show()

    def mountBucket(self):
        # get mount directory
        try:
            directory = str(self.lineEdit_mountPoint.text())
        except ValueError as e:
            self.addOutputText(f"Invalid mount path: {str(e)}")
            return
        # get config path
        configPath = os.path.join(self.getWorkingDir(), 'config.yaml')

        # on Windows, the mount directory should not exist (yet)
        if platform == 'win32':
            drive, tail = os.path.splitdrive(directory)
            # Only append the cloudfuse suffix if not mounting to a drive letter
            if not(drive and (tail == '' or tail in ['\\', '/'])):
                directory = os.path.join(directory, mountDirSuffix)

            if os.path.exists(directory):
                self.addOutputText(f"Directory {directory} already exists! Aborting new mount.")
                self.errorMessageBox(f"Error: Cloudfuse needs to create the directory {directory}, but it already exists!")
                return

        # do a dry run to validate options and credentials
        commandParts = [cloudfuseCli, 'mount', directory, f'--config-file={configPath}', '--dry-run']
        
        (stdOut, stdErr, exitCode, executableFound) = self.runCommand(commandParts)
        if not executableFound:
            self.addOutputText('cloudfuse.exe not found! Is it installed?')
            self.errorMessageBox('Error running cloudfuse CLI - Please re-install Cloudfuse.')
            return

        if exitCode != 0:
            self.addOutputText(stdErr)
            self.errorMessageBox('Mount failed: ' + stdErr)
            return

        if stdOut != '':
            self.addOutputText(stdOut)

        # now actually mount
        commandParts = [cloudfuseCli, 'mount', directory, f'--config-file={configPath}']
        if platform == "win32" and self.checkBox_remount.isChecked():
            commandParts.append('--enable-remount')
        (stdOut, stdErr, exitCode, executableFound) = self.runCommand(commandParts)
        if not executableFound:
            self.addOutputText('cloudfuse.exe not found! Is it installed?')
            self.errorMessageBox('Error running cloudfuse CLI - Please re-install Cloudfuse.')
            return

        if exitCode != 0:
            self.addOutputText(f"Error mounting container: {stdErr}")
            if stdErr.find('mount path exists') != -1:
                self.errorMessageBox('This container is already mounted at this directory.')
            else:
                self.errorMessageBox(f"Error mounting container - check the settings and try again\n{stdErr}")
            return

        if stdOut != '':
            self.addOutputText(stdOut)

        # wait for mount, then check that mount succeeded by verifying that the mount directory exists
        self.addOutputText('Verifying mount success...')
        def verifyMountSuccess():
            if platform == 'win32':
                success = os.path.exists(directory)
            else:
                success = os.path.ismount(directory)
            if not success:
                self.addOutputText(f"Failed to create mount directory {directory}")
                self.errorMessageBox('Mount failed. Please check error logs.')
            else:
                self.addOutputText('Successfully mounted container')
        QtCore.QTimer.singleShot(4000, verifyMountSuccess)

    def unmountBucket(self):
        directory = str(self.lineEdit_mountPoint.text())
        commandParts = []
        # TODO: properly handle unmount. This is relying on the line_edit not being changed by the user.
        drive, tail = os.path.splitdrive(directory)
        # Only append the cloudfuse suffix if not mounting to a drive letter
        if not(drive and (tail == '' or tail in ['\\', '/'])):
            directory = os.path.join(directory, mountDirSuffix)
        commandParts = [cloudfuseCli, 'unmount', directory]
        if platform != 'win32':
            commandParts.append('--lazy')
        if platform == "win32" and not self.checkBox_remount.isChecked():
            commandParts.append('--disable-remount')

        (stdOut, stdErr, exitCode, executableFound) = self.runCommand(commandParts)
        if not executableFound:
            self.addOutputText('cloudfuse.exe not found! Is it installed?')
            self.errorMessageBox('Error running cloudfuse CLI - Please re-install Cloudfuse.')
        elif exitCode != 0:
            self.addOutputText(f"Failed to unmount container: {stdErr}")
            self.errorMessageBox(f"Failed to unmount container: {stdErr}")
        else:
            self.addOutputText(f"Successfully unmounted container {stdErr}")

    # This function reads in the config file, modifies the components section, then writes the config file back
    def modifyPipeline(self):
        self.addOutputText('Validating configuration...')
        # Update the pipeline/components before mounting the target
        targetBucket = bucketOptions[self.dropDown_bucketSelect.currentIndex()]
        components = self.settings.get('components')
        if components != None:
            components[mountTargetComponent] = targetBucket
            self.settings['components'] = components
        else:
            workingDir = self.getWorkingDir()
            self.errorMessageBox(
                f"The components is missing in {workingDir}/config.yaml. Consider Going through the settings to create one.",
                'Components in config missing')
            return
        self.writeConfigFile(self.settings)

    # run command and return tuple:
    # (stdOut, stdErr, exitCode, executableFound)
    def runCommand(self, commandParts):
        if len(commandParts) < 1:
            # (stdOut, stdErr, exitCode, executableFound)
            return ('', '', -1, False)
        # run command
        try:
            process = subprocess.run(
                commandParts,
                capture_output=True,
                creationflags=subprocess.CREATE_NO_WINDOW if hasattr(subprocess, 'CREATE_NO_WINDOW') else 0)
            stdOut = process.stdout.decode().strip()
            stdErr = process.stderr.decode().strip()
            exitCode = process.returncode
            return (stdOut, stdErr, exitCode, True)
        except FileNotFoundError:
            return ('', '', -1, False)
        except PermissionError:
            return ('', '', -1, False)

    def addOutputText(self, textString):
        timestamp = datetime.datetime.now()
        # format into year/month/day, hour/minute/second
        formattedTimestamp = timestamp.strftime("%Y-%m-%d %H:%M:%S")
        self.textEdit_output.setText(f"{self.textEdit_output.toPlainText()}{formattedTimestamp}: {textString}\n")
        self.textEdit_output.repaint()
        self.textEdit_output.moveCursor(QtGui.QTextCursor.End)

    def errorMessageBox(self, messageString, titleString='Error'):
        msg = QtWidgets.QMessageBox()
        # Get the user's attention by popping open a new window
        msg.setWindowTitle(titleString)
        msg.setText(messageString)
        # Show the message box
        msg.exec()

    def chooseDriveLetter(self):
        unused_letters = get_unused_driver_letters()
        if not unused_letters:
            QtWidgets.QMessageBox.warning(self, 'No Drive Letters', 'No unused drive letters available.')
            return

        drive, ok = QtWidgets.QInputDialog.getItem(
            self,
            'Select Drive Letter',
            'Available drive letters:',
            unused_letters,
            0,
            False
        )
        if ok and drive:
            self.lineEdit_mountPoint.setText(f"{drive}")
            self.updateMountPointInSettings()

def get_unused_driver_letters():
    bitmask = ctypes.windll.kernel32.GetLogicalDrives()
    unused = []
    for letter in string.ascii_uppercase:
        if not ((bitmask & 1) or (letter == 'A' or letter == 'B')):
            unused.append(letter + ':')
        bitmask >>=1
    return unused
