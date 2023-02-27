from PySide6.QtCore import Qt
from PySide6 import QtWidgets
from PySide6.QtWidgets import QMainWindow
# import the custom class created with QtDesigner 
from ui_mountPrimaryWindow import Ui_primaryFUSEwindow

from mountSettings import mountSettingsWidget
from FCAz_simpleSettings import fcazSettingsWidget
from FCL_simpleSettings import fclSettingsWidget
from StAz_simpleSettings import stazSettingsWidget
from StL_simpleSettings import stlzSettingsWidget

from config_common import commonSettingsWidget

import subprocess
from sys import platform

pipeline = {
    "streaming" : 1,
    "filecaching" : 0
}

bucketOptions = {
    "Azure" : 1,
    "Lyve" : 0
}

class FUSEWindow(QMainWindow, Ui_primaryFUSEwindow):
    def __init__(self):
        super().__init__()
        self.setupUi(self)

        self.setWindowTitle("LyveCloud FUSE")


        # Set up the signals for this window
        
        #self.advancedSettings_action.triggered.connect(self.showSettingsWidget)
        self.setup_action.triggered.connect(self.showSettingsWidget)
        self.browse_button.clicked.connect(self.getFileDirInput)
        self.config_button.clicked.connect(self.showCommonSettingsWidget)
        self.mount_button.clicked.connect(self.mountBucket)
        self.unmount_button.clicked.connect(self.unmountBucket)

    # Define the slots that will be triggered when the signals in Qt are activated
    def showCommonSettingsWidget(self):
        self.settings = commonSettingsWidget()
        self.settings.show()


    def showSettingsWidget(self):

        mode = self.pipeline_select.currentIndex()
        mountTarget = self.bucket_select.currentIndex()

        # if mode == pipeline['filecaching'] and mountTarget == bucketOptions['Lyve']:
        #     self.settingsWindow = fclSettingsWidget()
        # elif mode == pipeline['filecaching'] and mountTarget == bucketOptions['Azure']:
        #     self.settingsWindow = fcazSettingsWidget()
        # elif mode == pipeline['streaming'] and mountTarget == bucketOptions['Lyve']:
        #     self.settingsWindow = stlzSettingsWidget()
        # elif mode == pipeline['streaming'] and mountTarget == bucketOptions['Azure']:
        #     self.settingsWindow = stazSettingsWidget()

        self.settingsWindow.show()
        # self.settingsWindow = mountSettingsWidget()
        # self.settingsWindow.show()

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
                # For future use to get output on Popen
                # for line in mount.stdout.readlines():    
            else:            
                mount = subprocess.run(["./lyvecloudfuse", "mount", directory, "--config-file=./config.yaml"])#,capture_output=True)
                if mount.returncode == 0:
                    # Print to the text edit window on success.  
                    self.output_textEdit.setText("Successfully mounted container\n")
                else:
                    #print(mount.stdout.decode())
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
            #print(unmount.stdout)
            if unmount.returncode == 0:
                self.output_textEdit.setText("Successfully unmounted container\n" + unmount.stdout.decode())
            else:
                self.output_textEdit.setText("!!Error unmounting container!!\n" + unmount.stdout.decode())
                msg.setWindowTitle("Error")
                msg.setText("Error unmounting container - check the logs")
                x = msg.exec()  # Show the message box
        except ValueError:
            pass