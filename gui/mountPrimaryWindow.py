# System imports
import subprocess
from sys import platform
import os
import yaml

# Import QT libraries
from PySide6.QtCore import Qt
from PySide6 import QtWidgets
from PySide6.QtWidgets import QMainWindow

# Import the custom class created with QtDesigner 
from ui_mountPrimaryWindow import Ui_primaryFUSEwindow
from lyve_config_common import lyveSettingsWidget
from azure_config_common import azureSettingsWidget
import sys
from io import StringIO

bucketOptions = ['s3storage', 'azstorage']
mountTargetComponent = 3
class FUSEWindow(QMainWindow, Ui_primaryFUSEwindow):
    def __init__(self):
        super().__init__()
        self.setupUi(self)

        self.setWindowTitle("LyveCloud FUSE")


        # Set up the signals for all the interactable intities
        self.button_browse.clicked.connect(self.getFileDirInput)
        self.button_config.clicked.connect(self.showSettingsWidget)
        self.button_mount.clicked.connect(self.mountBucket)
        self.button_unmount.clicked.connect(self.unmountBucket)

    # Define the slots that will be triggered when the signals in Qt are activated

    # There are unique settings per bucket selected for the pipeline, 
    # so we must use different widgets to show the different settings
    def showSettingsWidget(self):

        targetIndex = self.dropDown_bucketSelect.currentIndex()
        if bucketOptions[targetIndex] == 's3storage':
            self.settings = lyveSettingsWidget()
        else:
            self.settings = azureSettingsWidget()
        self.settings.setWindowModality(Qt.ApplicationModal)
        self.settings.show()

    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.lineEdit_mountPoint.setText('{}'.format(directory))

    #wrapper/helper for the service install and start.
    def serviceInstall(self, num):
        msg = QtWidgets.QMessageBox()
        isRunning = False
    
        if num == "win32":
            #put the 'mount =  subprocess.run()' commands wrapped in a exec()
            #use the completedProcess object in mount var to determine next steps 
            #if service already installed, run lyvecloudfuse.exe service start
            #if start successful, run lyvecloudfuse.exe service mount
            
            exec('mount = (subprocess.run([".\lyvecloudfuse.exe", "service", "install"], capture_output=True))', globals())         
            if (mount.returncode == 0 or mount.stderr.decode().find("lyvecloudfuse service already exists") != -1): #we found this message
                exec('mount = (subprocess.run([".\lyvecloudfuse.exe", "service", "start"], capture_output=True))', globals())
                if mount.stderr.decode().find("An instance of the service is already running.") != -1:
                    isRunning = True
                    self.textEdit_output.setText("!!The container is already mounted!!\n")# + mount.stdout.decode())
                    # Get the users attention by popping open a new window on an error
                    msg.setWindowTitle("Error")
                    msg.setText("This container is already mounted at this directory.")
                    # Show the message box
                    x = msg.exec()
                
                elif mount.returncode == 1: 

                    self.textEdit_output.setText("!!Error starting service before mounting container!!\n")# + mount.stdout.decode())
                    # Get the users attention by popping open a new window on an error
                    msg.setWindowTitle("Error")
                    msg.setText("Error mounting container - uninstall the service and try again")
                    # Show the message box
                    x = msg.exec()
                else:
                    isRunning = True
                    return True #started just fine
            else:
                if isRunning == True:
                    return True
                self.textEdit_output.setText("!!Error starting service to mount container!!\n")# + mount.stdout.decode())
                # Get the users attention by popping open a new window on an error
                msg.setWindowTitle("Error")
                msg.setText("Error starting service to mount container - check the settings and try again")
                # Show the message box
                x = msg.exec()
        else: 
            exec('mount = subprocess.run(["./lyvecloudfuse", "service", "install"], capture_output=True)', globals())#,capture_output=True)
            if (mount.returncode == 1 or mount.stderr.decode().find("lyvecloudfuse service already exists") != -1): #we found this message
                exec('mount = subprocess.run(["./lyvecloudfuse", "service", "install"], capture_output=True)', globals())#,capture_output=True)
                if mount.returncode != 0:
                    self.textEdit_output.setText("!!Error starting service before mounting container!!\n")# + mount.stdout.decode())
                    # Get the users attention by popping open a new window on an error
                    msg.setWindowTitle("Error")
                    msg.setText("Error mounting container - uninstall the service and try again")
                    # Show the message box
                    x = msg.exec() 
                else:
                    isRunning = True
                    return True #installed just fine or is already installed.
            else:
                if isRunning == True:
                    return True
                self.textEdit_output.setText("!!Error starting service to mount container!!\n")# + mount.stdout.decode())
                # Get the users attention by popping open a new window on an error
                msg.setWindowTitle("Error")
                msg.setText("Error starting service to mount container - check the settings and try again")
                # Show the message box
                x = msg.exec()

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
                # add lyveCloudFuse at the end of the directory 
                directory = directory+'/lyveCloudFuse'
                
                isRunning = self.serviceInstall(platform) #install and start the service
                exec('mount = (subprocess.run([".\lyvecloudfuse.exe", "service", "mount", directory, "--config-file=./config.yaml"], capture_output=True))', globals(), locals())
                if mount.returncode == 0:
                    self.textEdit_output.setText("Successfully mounted container\n")
                else:
                    if isRunning:
                        return
                    self.textEdit_output.setText("!!Error mounting container!!\n")# + mount.stdout.decode())
                    # Get the users attention by popping open a new window on an error
                    msg.setWindowTitle("Error")
                    msg.setText("Error mounting container - check the settings and try again")
                    # Show the message box
                    x = msg.exec()

                # TODO: For future use to get output on Popen
                #   for line in mount.stdout.readlines():    
            else:
                isRunning = self.serviceInstall(platform) #install and start the service
                exec('mount = subprocess.run(["./lyvecloudfuse", "mount", directory, "--config-file=./config.yaml"], capture_output=True)', globals(), locals())
                # Print to the text edit window the results of the mount
                if mount.returncode == 0:
                    self.textEdit_output.setText("Successfully mounted container\n")
                else:
                    if isRunning:
                        return
                    self.textEdit_output.setText("!!Error mounting container!!\n")# + mount.stdout.decode())
                    # Get the users attention by popping open a new window on an error
                    msg.setWindowTitle("Error")
                    msg.setText("Error mounting container - check the settings and try again")
                    # Show the message box
                    x = msg.exec()
        except ValueError:
            pass

    def unmountBucket(self):
        msg = QtWidgets.QMessageBox()
        try:
            exec('unmount = subprocess.run(["./lyvecloudfuse", "service", "stop"], capture_output=True)', globals())
            # Print to the text edit window the results of the unmount
            if unmount.returncode == 0:
                self.textEdit_output.setText("Successfully unmounted container\n" + unmount.stdout.decode())
            else:
                self.textEdit_output.setText("!!Error unmounting container!!\n" + unmount.stdout.decode())
                msg.setWindowTitle("Error")
                msg.setText("Error unmounting container - check the logs")
                # Show the message box
                x = msg.exec()
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