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

    # Wrapper/helper for the service install and start.
    def windowsServiceInstall(self):
        msg = QtWidgets.QMessageBox()

        # Use the completedProcess object in mount var to determine next steps 
        #   if service already installed, run cloudfuse.exe service start
        #   if start successful, run cloudfuse.exe service mount

        try:
            windowsServiceCmd = subprocess.run([".\cloudfuse.exe", "service", "install"], capture_output=True, check=True)
        except:
            return False

        if windowsServiceCmd.returncode == 0 or windowsServiceCmd.stderr.decode().find("cloudfuse service already exists") != -1: #we found this message
            windowsServiceCmd = (subprocess.run([".\cloudfuse.exe", "service", "start"], capture_output=True))
            if windowsServiceCmd.stderr.decode().find("An instance of the service is already running.") != -1:
                return True
            elif windowsServiceCmd.returncode == 1: 
                self.textEdit_output.setText("!!Error starting service before mounting container!!\n")# + mount.stdout.decode())
                # Get the users attention by popping open a new window on an error
                msg.setWindowTitle("Error")
                msg.setText("Error mounting container - Run this application as administrator. uninstall the service and try again")
                # Show the message box
                msg.exec()
                return False
            else:
                # Started just fine
                return True 
        else:
            self.textEdit_output.setText("!!Error installing service to mount container!!\n")# + mount.stdout.decode())
            # Get the users attention by popping open a new window on an error
            msg.setWindowTitle("Error")
            msg.setText("Error installing service to mount container - Run application as administrator and try again")
            # Show the message box
            msg.exec()
            return False
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
            
            if platform == "win32":
                # Windows mount has a quirk where the folder shouldn't exist yet,
                #   add CloudFuse at the end of the directory 
                directory = directory+'\cloudFuse'
                
                # Install and start the service
                isRunning = self.windowsServiceInstall()  
            
                if isRunning:
                    # ".\cloudfuse.exe", "service", "mount", directory, "--config-file=.\config.yaml"
                    mount = (subprocess.run([".\cloudfuse.exe","service","mount", directory,"--config-file=.\config.yaml"],capture_output=True))
                    if mount.returncode == 0:
                        self.textEdit_output.setText("Successfully mounted container\n")
                    elif mount.stderr.decode().find("mount path exists") != -1:
                        self.textEdit_output.setText("!!The container is already mounted!!\n")# + mount.stdout.decode())
                        # Get the users attention by popping open a new window on an error
                        msg.setWindowTitle("Error")
                        msg.setText("This container is already mounted at this directory.")
                        # Show the message box
                        msg.exec()
                else:
                    self.textEdit_output.setText("!!Error mounting container!!\n")# + mount.stdout.decode())
                    # Get the users attention by popping open a new window on an error
                    msg.setWindowTitle("Error")
                    msg.setText("Error mounting container - check the settings and try again")
                    # Show the message box
                    msg.exec()

                # TODO: For future use to get output on Popen
                # for line in mount.stdout.readlines():    
            else:
                # Create the mount command to send to subprocess. If shell=True is set and the command is not in one string
                #   the subprocess will interpret the additional arguments as separate commands. 
                cmd = "./cloudfuse mount " + directory + " --config-file=./config.yaml"
                mount = subprocess.run([cmd], shell=True, capture_output=True)

                # Print to the text edit window the results of the mount
                if mount.returncode == 0:
                    self.textEdit_output.setText("Successfully mounted container\n")
                else:
                    self.textEdit_output.setText("!!Error mounting container!!\n" + mount.stderr.decode())
                    # Get the users attention by popping open a new window on an error
                    msg.setWindowTitle("Error")
                    msg.setText("Error mounting container - check the settings and try again\n" + mount.stderr.decode())
                    # Show the message box
                    msg.exec()
        except ValueError:
            pass

    def unmountBucket(self):
        msg = QtWidgets.QMessageBox()
        directory = str(self.lineEdit_mountPoint.text())
        # TODO: properly handle unmount. This is relying on the line_edit not being changed by the user.
        try:
            if platform == "win32":
                # for windows, 'cloudfuse' was added to the directory so add it back in for umount
                directory = directory+'/cloudFuse'
                unmount = subprocess.run([".\cloudfuse.exe", "service", "unmount", directory], shell=True, capture_output=True)
            else:
                # Create the mount command to send to subprocess. If shell=True is set and the command is not in one string
                #   the subprocess will interpret the additional arguments as separate commands.
                cmd = "./cloudfuse unmount " + directory
                unmount = subprocess.run([cmd], shell=True, capture_output=True)

            # Print to the text edit window the results of the unmount
            if unmount.returncode == 0:
                self.textEdit_output.setText("Successfully unmounted container\n" + unmount.stderr.decode())
            else:
                self.textEdit_output.setText("!!Error unmounting container!!\n" + unmount.stderr.decode())
                msg.setWindowTitle("Error")
                msg.setText("Error unmounting container - check the logs\n" + unmount.stderr.decode())
                # Show the message box
                msg.exec()
        except ValueError:
            pass
        
        
    # This function reads in the config file, modifies the components section, then writes the config file back
    def modifyPipeline(self,target):
        currentDir = os.getcwd()
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