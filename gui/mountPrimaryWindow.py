# Import QT libraries
from PySide6.QtCore import Qt
from PySide6 import QtWidgets
from PySide6.QtWidgets import QMainWindow

# System imports
import subprocess
from sys import platform

# Import the custom class created with QtDesigner 
from ui_mountPrimaryWindow import Ui_primaryFUSEwindow
from config_common import lyveSettingsWidget
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


        # Set up the signals for this window
        self.setup_action.triggered.connect(self.showSettingsWidget)
        self.browse_button.clicked.connect(self.getFileDirInput)
        self.config_button.clicked.connect(self.showSettingsWidget)
        self.mount_button.clicked.connect(self.mountBucket)
        self.unmount_button.clicked.connect(self.unmountBucket)

    # Define the slots that will be triggered when the signals in Qt are activated

    def showSettingsWidget(self):

        mountTarget = self.bucket_select.currentIndex()
        
        if mountTarget == bucketOptions['Lyve']:
            self.settings = lyveSettingsWidget()
        else:
            self.settings = azureSettingsWidget()
        self.settings.show()


    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.mountPoint_input.setText('{}'.format(directory))

    def mountBucket(self):
        msg = QtWidgets.QMessageBox()
        if self.bucket_select.currentIndex() != bucketOptions["Azure"]:
            msg.setWindowTitle("Error")
            msg.setText("S3 bucket not enabled yet, use an Azure bucket for now")
            x = msg.exec()  # Show the message box
            return
        try:
            directory = str(self.mountPoint_input.text())
            
            if platform == "win32":
                directory = directory+'/lyveCloudFuse'
                mount = subprocess.Popen([".\lyvecloudfuse.exe", "mount", directory, "--config-file=.\config.yaml"], stdout=subprocess.PIPE)
                # TODO: For future use to get output on Popen
                # for line in mount.stdout.readlines():    
            else:            
                mount = subprocess.run(["./lyvecloudfuse", "mount", directory, "--config-file=./config.yaml"])#,capture_output=True)
                if mount.returncode == 0:
                    # Print to the text edit window on success.  
                    self.output_textEdit.setText("Successfully mounted container\n")
                else:
                    self.output_textEdit.setText("!!Error mounting container!!\n")# + mount.stdout.decode())
                    
                    # Get the users attention by popping open a new window on an error
                    msg.setWindowTitle("Error")
                    msg.setText("Error mounting container - check the settings and try again")
                    x = msg.exec()  # Show the message box
            

        except ValueError:
            pass

    def unmountBucket(self):
        msg = QtWidgets.QMessageBox()
        try:
            unmount = subprocess.run(["./lyvecloudfuse", "unmount", "all"],capture_output=True)
            if unmount.returncode == 0:
                self.output_textEdit.setText("Successfully unmounted container\n" + unmount.stdout.decode())
            else:
                self.output_textEdit.setText("!!Error unmounting container!!\n" + unmount.stdout.decode())
                msg.setWindowTitle("Error")
                msg.setText("Error unmounting container - check the logs")
                x = msg.exec()  # Show the message box
        except ValueError:
            pass