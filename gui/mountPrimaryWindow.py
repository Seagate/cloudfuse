# System imports
import subprocess
from sys import platform
import os
import yaml

# Import QT libraries
from PySide6.QtCore import Qt
from PySide6 import QtWidgets, QtGui
from PySide6.QtWidgets import QMainWindow

# Import the custom class created with QtDesigner 
from ui_mountPrimaryWindow import Ui_primaryFUSEwindow
from s3_config_common import s3SettingsWidget
from azure_config_common import azureSettingsWidget
from aboutPage import aboutPage
from under_Construction import underConstruction
from common_qt_functions import widgetCustomFunctions as widgetFuncs

bucketOptions = ['s3storage', 'azstorage']
mountTargetComponent = 3

class FUSEWindow(QMainWindow, Ui_primaryFUSEwindow):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle("Cloud FUSE")
        
        if platform == 'win32':
            # Windows directory and filename conventions:
            #   https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#file-and-directory-names
            # Disallow the following [<,>,.,",|,?,*] - note, we still need directory characters to declare a path
            self.lineEdit_mountPoint.setValidator(QtGui.QRegularExpressionValidator(r'^[^<>."|?\0*]*$',self))
        else:
            # Allow anything BUT Nul
            # Note: Different versions of Python don't like the embedded null character, send in the raw string instead
            self.lineEdit_mountPoint.setValidator(QtGui.QRegularExpressionValidator(r'^[^\0]*$',self))
       
        # Set up the signals for all the interactable intities
        self.button_browse.clicked.connect(self.getFileDirInput)
        self.button_config.clicked.connect(self.showSettingsWidget)
        self.button_mount.clicked.connect(self.mountBucket)
        self.button_unmount.clicked.connect(self.unmountBucket)
        self.actionAbout_Qt.triggered.connect(self.showAboutQtPage)
        self.actionAbout_CloudFuse.triggered.connect(self.showAboutCloudFusePage)
        self.action_debugHealthMonitor.triggered.connect(self.showUnderConstructionPage)
        self.action_debugLogging.triggered.connect(self.showUnderConstructionPage)
        self.action_debugTesting.triggered.connect(self.showUnderConstructionPage)
        
        if platform == "win32":
            self.lineEdit_mountPoint.setToolTip("Designate a new location to mount the bucket, do not create the directory")
            self.button_browse.setToolTip("Browse to a new location but don't create a new directory")
        else:
            self.lineEdit_mountPoint.setToolTip("Designate a location to mount the bucket - the directory must already exist")
            self.button_browse.setToolTip("Browse to a pre-existing directory")

    # Define the slots that will be triggered when the signals in Qt are activated

    # There are unique settings per bucket selected for the pipeline, 
    #   so we must use different widgets to show the different settings
    def showSettingsWidget(self):

        targetIndex = self.dropDown_bucketSelect.currentIndex()
        if bucketOptions[targetIndex] == 's3storage':
            self.settings = s3SettingsWidget()
        else:
            self.settings = azureSettingsWidget()
        self.settings.setWindowModality(Qt.ApplicationModal)
        self.settings.show()

    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.lineEdit_mountPoint.setText('{}'.format(directory))


    # Display the pre-baked about QT messagebox
    def showAboutQtPage(self):
        QtWidgets.QMessageBox.aboutQt(self, "About QT")

    # Display the custom dialog box for the cloudfuse 'about' page.
    def showAboutCloudFusePage(self):
        self.page = aboutPage()
        self.page.show()

    def showUnderConstructionPage(self):
        self.page = underConstruction()
        self.page.show()

    # Wrapper/helper for the service install and start.
    def windowsServiceInstall(self):
        # install the service
        (stdOut, stdErr, exitCode, executableFound) = self.runCommand("cloudfuse.exe service install")
        if not executableFound:
            self.addOutputText("cloudfuse.exe not found! Is it installed?")
            self.errorMessageBox("Error running cloudfuse CLI - Please re-install Cloudfuse.")
            return False
        if exitCode != 0:
            # check if this is a permissions issue
            if stdErr.toLower().find('admin') != -1:
                self.addOutputText(stdErr)
                self.errorMessageBox("Error mounting container - Please re-launch this application as administrator.")
                return False
            # check if the request was redundant
            if stdErr.toLower().find('already') != -1:
                return True
            else:
                # stop on any other error
                self.addOutputText(stdErr)
                return False
        # start the service
        (stdOut, stdErr, exitCode, executableFound) = self.runCommand("cloudfuse.exe service start")
        if not executableFound:
            self.addOutputText("cloudfuse.exe not found! Is it installed?")
            self.errorMessageBox("Error running cloudfuse CLI - Please re-install Cloudfuse.")
            return False
        if exitCode != 0:
            # check if this is a permissions issue
            if stdErr.toLower().find('admin') != -1:
                self.addOutputText(stdErr)
                self.errorMessageBox("Error mounting container - Please re-launch this application as administrator.")
                return False
            # check if the request was redundant
            if stdErr.toLower().find('already') != -1:
                return True
            else:
                # stop on any other error
                self.addOutputText(stdErr)
                return False
        return True

    def mountBucket(self):
        msg = QtWidgets.QMessageBox()
        
        # Update the pipeline/components before mounting the target
        targetIndex = self.dropDown_bucketSelect.currentIndex() 
        success = self.modifyPipeline(bucketOptions[targetIndex])
        if not success:
            # Don't try mounting the container since the config file couldn't be modified for the pipeline setting
            return
        
        try:
            directory = str(self.lineEdit_mountPoint.text())
        except ValueError as e:
            self.addOutputText(f"Invalid mount path: {str(e)}")
            return
        
        if platform == "win32":
            # Windows mount has a quirk where the folder shouldn't exist yet,
            #   add CloudFuse at the end of the directory 
            directory = directory+'\cloudFuse'
            
            # Install and start the service
            isRunning = self.windowsServiceInstall()  
        
            if isRunning:
                (stdOut, stdErr, exitCode, executableFound) = self.runCommand(f"cloudfuse.exe service mount {directory} --config-file=config.yaml")
                if not executableFound:
                    self.addOutputText("cloudfuse.exe not found! Is it installed?")
                    self.errorMessageBox("Error running cloudfuse CLI - Please re-install Cloudfuse.")
                elif exitCode != 0:
                    self.addOutputText(stdErr)
                    # check if this is a permissions issue
                    if stdErr.toLower().find('admin') != -1:
                        self.errorMessageBox("Error mounting container - Please re-launch this application as administrator.")
                    elif stdErr.toLower().find("mount path exists") != -1:
                        self.errorMessageBox("This container is already mounted at this directory.")
                else:
                    self.addOutputText("Successfully mounted container")
        else:
            (stdOut, stdErr, exitCode, executableFound) = self.runCommand(f"./cloudfuse mount {directory} --config-file=./config.yaml")
            if exitCode != 0:
                self.addOutputText(f"Error mounting container: {stdErr}")
                self.errorMessageBox(f"Error mounting container - check the settings and try again\n{stdErr}")
            else:
                self.addOutputText("Successfully mounted container\n")

    def unmountBucket(self):
        directory = str(self.lineEdit_mountPoint.text())
        commandString = ""
        # TODO: properly handle unmount. This is relying on the line_edit not being changed by the user.
        
        if platform == "win32":
            # for windows, 'cloudfuse' was added to the directory so add it back in for umount
            directory = directory+'/cloudFuse'
            commandString = f"cloudfuse.exe service unmount {directory}"
        else:
            commandString = f"./cloudfuse unmount --lazy {directory}"
        
        (stdOut, stdErr, exitCode, executableFound) = self.runCommand(commandString)
        if not executableFound:
            self.addOutputText("cloudfuse.exe not found! Is it installed?")
            self.errorMessageBox("Error running cloudfuse CLI - Please re-install Cloudfuse.")
        elif exitCode != 0:
            self.addOutputText(f"Failed to unmount container: {stdErr}")
            self.errorMessageBox(f"Failed to unmount container: {stdErr}")
        else:
            self.addOutputText(f"Successfully unmounted container\n{stdErr}")

    # This function reads in the config file, modifies the components section, then writes the config file back
    def modifyPipeline(self,target):

        currentDir = widgetFuncs.getCurrentDir(self)
        errMsg = QtWidgets.QMessageBox()
        
        # Read in the configs as a dictionary. Notify user if failed
        try:
            with open(currentDir+'/config.yaml', 'r') as file:
                configs = yaml.safe_load(file)
        except:
            errMsg.setWindowTitle("Could not read config file")
            errMsg.setText(f"Could not read the config file in {currentDir}. Consider going through the settings for selected target.")
            errMsg.exec()
            return False
        
        # Modify the components (pipeline) in the config file. 
        #   If the components are not present, there's a chance the configs are wrong. Notify user.

        components = configs.get('components')
        if components != None:
            components[mountTargetComponent] = target
            configs['components'] = components
        else:
            errMsg.setWindowTitle("Components in config missing")
            errMsg.setText(f"The components is missing in {currentDir}/config.yaml. Consider Going through the settings to create one.")
            errMsg.exec()            
            return False
        
        # Write the config file with the modified components 
        try:
            with open(currentDir+'/config.yaml','w') as file:
                yaml.safe_dump(configs,file)
        except:
            errMsg.setWindowTitle("Could not modify config file")
            errMsg.setText(f"Could not modify {currentDir}/config.yaml.")
            errMsg.exec()
            return False
        
        # If nothing failed so far, return true to proceed to the mount phase
        return True
    
    # run command and return tuple:
    # (stdOut, stdErr, exitCode, executableFound)
    def runCommand(self, commandString):
        # cut commandString into parts
        commandParts = commandString.split(' ')
        if len(commandParts) < 1:
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
    
    def addOutputText(self, textString):
        self.textEdit_output.setText(f"{self.textEdit_output.toPlainText()}{textString}\n")
    
    def errorMessageBox(self, messageString):
        msg = QtWidgets.QMessageBox()
        # Get the user's attention by popping open a new window
        msg.setWindowTitle("Error")
        msg.setText(messageString)
        # Show the message box
        msg.exec()