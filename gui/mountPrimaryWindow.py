from PySide6.QtCore import Qt
from PySide6 import QtWidgets
from PySide6.QtWidgets import QMainWindow
# import the custom class created with QtDesigner 
from ui_mountPrimaryWindow import Ui_primaryFUSEwindow
from mountSettings import mountSettingsWidget
import subprocess

pipeline = {
    "streaming" : 0,
    "filecaching" : 1
}

butcketOptions = {
    "Azure" : 0,
    "S3" : 1
}

class FUSEWindow(QMainWindow, Ui_primaryFUSEwindow):
    def __init__(self):
        super().__init__()
        self.setupUi(self)

        self.setWindowTitle("LyveCloud FUSE")


        # Set up the signals for this window
        
        self.advancedSettings_action.triggered.connect(self.show_settings_widget)
        self.browse_button.clicked.connect(self.get_File_Directory_Input)
        self.mount_button.clicked.connect(self.mount_Bucket)
    
    # Define the slots that will be triggered when the signals in Qt are activated
    def show_settings_widget(self):
        self.settingsWindow = mountSettingsWidget()
        self.settingsWindow.show()

    def get_File_Directory_Input(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.mountPoint_input.setText('{}'.format(directory))

    def mount_Bucket(self):
        msg = QtWidgets.QMessageBox()
        if self.bucket_select.currentIndex() != butcketOptions["Azure"]:
            msg.setWindowTitle("Error")
            msg.setText("S3 bucket not enabled yet, use an Azure bucket for now")
            x = msg.exec()  # Show the message box
            return
        try:
            directory = str(self.mountPoint_input.text())
            mount = subprocess.run(["./lyvecloudfuse", "mount", directory, "--config-file=./config.yaml"])

            if mount.returncode == 0:
                # Print to the text edit window on success.  
                self.output_textEdit.setText("Successfully mounted container")
            else:
                self.output_textEdit.setText('Error mounting container')
                
                # Get the users attention by popping open a new window on an error
                msg.setWindowTitle("Error")
                msg.setText("Error mounting container - check the settings and try again")
                x = msg.exec()  # Show the message box

        except ValueError:
            pass