from PySide6.QtCore import Qt
from PySide6 import QtWidgets
from PySide6.QtWidgets import QMainWindow
# import the custom class created with QtDesigner 
from ui_mountPrimaryWindow import Ui_primaryFUSEwindow
from mountSettings import mountSettingsWidget
import subprocess

class FUSEWindow(QMainWindow, Ui_primaryFUSEwindow):
    def __init__(self):
        super().__init__()
        self.setupUi(self)

        self.setWindowTitle("LyveCloud FUSE")


        # Set up the signals for this window
        
        # self.mount_button.clicked.connect(self.show_settings_widget) 
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
        try:
            directory = str(self.mountPoint_input.text())
            print(directory)
            mount = subprocess.run(["blobfuse2", "mount", "all", directory, "--config-file=./config.yaml"])
            
            # for some reason, the blobfuse2 aliased to azure-storage-fuse won't work
            # mount = subprocess.run(["azure-storage-fuse"])
            if mount.returncode == 0:
                # TextEdit kind of sucks, use Qmessage 
                self.output_textEdit.setText("Success")#QtWidgets.QMessageBox()
                msg.setWindowTitle("Success")
                msg.setText("Successfully mounted container")
                x = msg.exec()  # this will show our messagebox

            else:
                # TextEdit kind of sucks, use Qmessage
                self.output_textEdit.setText('error mountint container' + str(mount.returncode))
               
                msg = QtWidgets.QMessageBox()
                msg.setWindowTitle("Error")
                msg.setText("Error mounting container")
                x = msg.exec()  # this will show our messagebox

        except ValueError:
            pass