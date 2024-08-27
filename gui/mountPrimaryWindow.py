# Licensed under the MIT License <http://opensource.org/licenses/MIT>.
#
# Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
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
import time
import yaml
from shutil import which

# Import QT libraries
from PySide6.QtCore import Qt, QSettings
from PySide6 import QtWidgets, QtGui, QtCore
from PySide6.QtWidgets import QMainWindow

# Import the custom class created with QtDesigner
from ui_mountPrimaryWindow import Ui_primaryFUSEwindow
from s3_config_common import s3SettingsWidget
from azure_config_common import azureSettingsWidget
from aboutPage import aboutPage
from under_Construction import underConstruction
from common_qt_functions import widgetCustomFunctions as widgetFuncs, defaultSettingsManager as settingsManager

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

class FUSEWindow(QMainWindow, Ui_primaryFUSEwindow):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle("Cloud FUSE")
        self.initMountPoint()
        self.checkConfigDirectory()
        self.textEdit_output.setReadOnly(True)
        self.settings = settingsManager()
        if platform == 'win32':
            # Windows directory and filename conventions:
            #   https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#file-and-directory-names
            # Disallow the following [<,>,.,",|,?,*] - note, we still need directory characters to declare a path
            self.lineEdit_mountPoint.setValidator(QtGui.QRegularExpressionValidator(r'^[^<>."|?\0*]*$',self))
        else:
            # Allow anything BUT Nul
            # Note: Different versions of Python don't like the embedded null character, send in the raw string instead
            self.lineEdit_mountPoint.setValidator(QtGui.QRegularExpressionValidator(r'^[^\0]*$',self))

        # Set up the signals for all the interactive entities
        self.button_browse.clicked.connect(self.getFileDirInput)
        self.button_config.clicked.connect(self.showSettingsWidget)
        self.button_mount.clicked.connect(self.mountBucket)
        self.button_unmount.clicked.connect(self.unmountBucket)
        self.actionAbout_Qt.triggered.connect(self.showAboutQtPage)
        self.actionAbout_CloudFuse.triggered.connect(self.showAboutCloudFusePage)
        self.action_debugHealthMonitor.triggered.connect(self.showUnderConstructionPage)
        self.action_debugLogging.triggered.connect(self.showUnderConstructionPage)
        self.action_debugTesting.triggered.connect(self.showUnderConstructionPage)
        self.lineEdit_mountPoint.editingFinished.connect(self.updateMountPointInSettings)
        self.dropDown_bucketSelect.currentIndexChanged.connect(self.modifyPipeline)
        if platform == "win32":
            self.lineEdit_mountPoint.setToolTip("Designate a new location to mount the bucket, do not create the directory")
            self.button_browse.setToolTip("Browse to a new location but don't create a new directory")
        else:
            self.lineEdit_mountPoint.setToolTip("Designate a location to mount the bucket - the directory must already exist")
            self.button_browse.setToolTip("Browse to a pre-existing directory")

    def checkConfigDirectory(self):
        workingDir = widgetFuncs.getWorkingDir(self)
        if not os.path.isdir(workingDir):
            try:
                os.mkdir(workingDir)
            except OSError as e:
                self.addOutputText(f"Failed to make own path: {str(e)}")

    def initMountPoint(self):
        try:
            directory = self.settings.value("mountPoint")
            self.lineEdit_mountPoint.setText(directory)
        except:
            # Nothing in the settings for mountDir, leave mountPoint blank
            return

    def updateMountPointInSettings(self):
        try:
            directory = str(self.lineEdit_mountPoint.text())
            self.settings.setValue("mountPoint", directory)
        except:
            # Couldn't update the settings
            return

    # Define the slots that will be triggered when the signals in Qt are activated

    # There are unique settings per bucket selected for the pipeline,
    #   so we must use different widgets to show the different settings
    def showSettingsWidget(self):
        targetIndex = self.dropDown_bucketSelect.currentIndex()
        if bucketOptions[targetIndex] == 's3storage':
            self.setConfigs = s3SettingsWidget(self.settings.allMountSettings)
        else:
            self.setConfigs = azureSettingsWidget(self.settings.allMountSettings)
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
        QtWidgets.QMessageBox.aboutQt(self, "About QT")

    # Display the custom dialog box for the cloudfuse 'about' page.
    def showAboutCloudFusePage(self):
        commandParts = [cloudfuseCli, '--version']
        (stdOut, stdErr, exitCode, executableFound) = self.runCommand(commandParts)

        if not executableFound:
            cloudfuseVersion = 'Cloudfuse program not present'
        elif stdOut != "":
            cloudfuseVersion = stdOut
        else:
            cloudfuseVersion = 'Cloudfuse version not found'

        self.page = aboutPage(cloudfuseVersion)
        self.page.show()

    def showUnderConstructionPage(self):
        self.page = underConstruction()
        self.page.show()

    def mountBucket(self):
        # get mount directory
        try:
            directory = str(self.lineEdit_mountPoint.text())
        except ValueError as e:
            self.addOutputText(f"Invalid mount path: {str(e)}")
            return
        directory = os.path.join(directory, mountDirSuffix)
        # get config path
        configPath = os.path.join(widgetFuncs.getWorkingDir(self), 'config.yaml')

        # on Windows, the mount directory should not exist (yet)
        if platform == "win32":
            if os.path.exists(directory):
                self.addOutputText(f"Directory {directory} already exists! Aborting new mount.")
                self.errorMessageBox(f"Error: Cloudfuse needs to create the directory {directory}, but it already exists!")
                return

        # do a dry run to validate options and credentials
        commandParts = [cloudfuseCli, 'mount', directory, f'--config-file={configPath}', '--dry-run']
        (stdOut, stdErr, exitCode, executableFound) = self.runCommand(commandParts)
        if not executableFound:
            self.addOutputText("cloudfuse.exe not found! Is it installed?")
            self.errorMessageBox("Error running cloudfuse CLI - Please re-install Cloudfuse.")
            return

        if exitCode != 0:
            self.addOutputText(stdErr)
            self.errorMessageBox("Mount failed: " + stdErr)
            return

        if stdOut != "":
            self.addOutputText(stdOut)

        # now actually mount
        commandParts = [cloudfuseCli, 'mount', directory, f'--config-file={configPath}']
        (stdOut, stdErr, exitCode, executableFound) = self.runCommand(commandParts)
        if not executableFound:
            self.addOutputText("cloudfuse.exe not found! Is it installed?")
            self.errorMessageBox("Error running cloudfuse CLI - Please re-install Cloudfuse.")
            return

        if exitCode != 0:
            self.addOutputText(f"Error mounting container: {stdErr}")
            if stdErr.find("mount path exists") != -1:
                self.errorMessageBox("This container is already mounted at this directory.")
            else:
                self.errorMessageBox(f"Error mounting container - check the settings and try again\n{stdErr}")
            return

        if stdOut != "":
            self.addOutputText(stdOut)

        # wait for mount, then check that mount succeeded by verifying that the mount directory exists
        self.addOutputText("Verifying mount success...")
        def verifyMountSuccess():
            if platform == 'win32':
                success = os.path.exists(directory)
            else:
                success = os.path.ismount(directory)
            if not success:
                self.addOutputText(f"Failed to create mount directory {directory}")
                self.errorMessageBox("Mount failed. Please check error logs.")
            else:
                self.addOutputText("Successfully mounted container")
        QtCore.QTimer.singleShot(4000, verifyMountSuccess)

    def unmountBucket(self):
        directory = str(self.lineEdit_mountPoint.text())
        commandParts = []
        # TODO: properly handle unmount. This is relying on the line_edit not being changed by the user.
        directory = os.path.join(directory, mountDirSuffix)
        commandParts = [cloudfuseCli, "unmount", directory]
        if platform != "win32":
            commandParts.append("--lazy")

        (stdOut, stdErr, exitCode, executableFound) = self.runCommand(commandParts)
        if not executableFound:
            self.addOutputText("cloudfuse.exe not found! Is it installed?")
            self.errorMessageBox("Error running cloudfuse CLI - Please re-install Cloudfuse.")
        elif exitCode != 0:
            self.addOutputText(f"Failed to unmount container: {stdErr}")
            self.errorMessageBox(f"Failed to unmount container: {stdErr}")
        else:
            self.addOutputText(f"Successfully unmounted container {stdErr}")

    # This function reads in the config file, modifies the components section, then writes the config file back
    def modifyPipeline(self):

        self.addOutputText("Validating configuration...")
        # Update the pipeline/components before mounting the target

        targetBucket = bucketOptions[self.dropDown_bucketSelect.currentIndex()]
        workingDir = widgetFuncs.getWorkingDir(self)

        # Read in the configs as a dictionary. Notify user if failed
        try:
            with open(workingDir+'/config.yaml', 'r') as file:
                configs = yaml.safe_load(file)
        except:
            self.errorMessageBox(
                f"Could not read the config file in {workingDir}. Consider going through the settings for selected target.",
                "Could not read config file")
            return

        # Modify the components (pipeline) in the config file.
        #   If the components are not present, there's a chance the configs are wrong. Notify user.

        components = configs.get('components')
        if components != None:
            components[mountTargetComponent] = targetBucket
            configs['components'] = components
        else:
            self.errorMessageBox(
                f"The components is missing in {workingDir}/config.yaml. Consider Going through the settings to create one.",
                "Components in config missing")
            return

        # Write the config file with the modified components
        try:
            with open(workingDir+'/config.yaml','w') as file:
                yaml.safe_dump(configs,file)
        except:
            self.errorMessageBox(f"Could not modify {workingDir}/config.yaml.", "Could not modify config file")
            return

    # run command and return tuple:
    # (stdOut, stdErr, exitCode, executableFound)
    def runCommand(self, commandParts):
        if len(commandParts) < 1:
            # (stdOut, stdErr, exitCode, executableFound)
            return ('', '', -1, False)
        # run command
        try:
            process = subprocess.run(commandParts, capture_output=True)
            stdOut = process.stdout.decode().strip()
            stdErr = process.stderr.decode().strip()
            exitCode = process.returncode
            return (stdOut, stdErr, exitCode, True)
        except FileNotFoundError:
            return ('', '', -1, False)
        except PermissionError:
            return ('', '', -1, False)

    def addOutputText(self, textString):
        self.textEdit_output.setText(f"{self.textEdit_output.toPlainText()}{textString}\n")
        self.textEdit_output.repaint()
        self.textEdit_output.moveCursor(QtGui.QTextCursor.End)

    def errorMessageBox(self, messageString, titleString="Error"):
        msg = QtWidgets.QMessageBox()
        # Get the user's attention by popping open a new window
        msg.setWindowTitle(titleString)
        msg.setText(messageString)
        # Show the message box
        msg.exec()