# System imports
import subprocess
from sys import platform

# Import QT libraries
from PySide6.QtCore import Qt
from PySide6 import QtWidgets
from PySide6.QtWidgets import QMainWindow

# Import the custom class created with QtDesigner 
from ui_mountPrimaryWindow import Ui_primaryFUSEwindow
from lyve_config_common import lyveSettingsWidget
from azure_config_common import azureSettingsWidget

bucketOptions = {
    "Lyve" : 0,
    "Azure" : 1
}

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

        mountTarget = self.dropDown_bucketSelect.currentIndex()
        
        if mountTarget == bucketOptions['Lyve']:
            self.settings = lyveSettingsWidget()
        else:
            self.settings = azureSettingsWidget()
        self.settings.setWindowModality(Qt.ApplicationModal)
        self.settings.show()

    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.lineEdit_mountPoint.setText('{}'.format(directory))

    def mountBucket(self):
        msg = QtWidgets.QMessageBox()
        
        # TODO: If target is set, the pipeline needs to change the target to azure or s3, 
        #   at the moment the settings widgets change the pipeline, 
        #   but if the user just wants to change the target and nothing else they wouldn't go into the settings. 

        try:
            directory = str(self.lineEdit_mountPoint.text())
            
            if platform == "win32":
                # Windows mount has a quirk where the folder shouldn't exist yet,
                # add lyveCloudFuse at the end of the directory 
                directory = directory+'/lyveCloudFuse'
                mount = subprocess.Popen([".\lyvecloudfuse.exe", "mount", directory, "--config-file=.\config.yaml"], stdout=subprocess.PIPE)
                
                # TODO: For future use to get output on Popen
                #   for line in mount.stdout.readlines():    
            else:            
                mount = subprocess.run(["./lyvecloudfuse", "mount", directory, "--config-file=./config.yaml"])#,capture_output=True)
                
                # Print to the text edit window the results of the mount
                if mount.returncode == 0:
                    self.textEdit_output.setText("Successfully mounted container\n")
                else:
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
            unmount = subprocess.run(["./lyvecloudfuse", "unmount", "all"],capture_output=True)
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