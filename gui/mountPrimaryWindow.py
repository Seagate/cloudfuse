from PySide6.QtCore import Qt
from PySide6 import QtWidgets
from PySide6.QtWidgets import QMainWindow
# import the custom class created with QtDesigner 
from ui_mountPrimaryWindow import Ui_primaryFUSEwindow
from mountSettings import mountSettingsWidget


class FUSEWindow(QMainWindow, Ui_primaryFUSEwindow):
    def __init__(self):
        super().__init__()
        self.setupUi(self)

        self.setWindowTitle("LyveCloud FUSE")


        # Set up the signals for this window
        
        # self.mount_button.clicked.connect(self.show_settings_widget) 
        self.advancedSettings_action.triggered.connect(self.show_settings_widget)

    
    # Define the slots that will be triggered when the signals in Qt are activated
    def show_settings_widget(self):
        self.settingsWindow = mountSettingsWidget()
        self.settingsWindow.show()





        #Browse button code
    #def openFileDialog(self):
    #directory = str(QtWidgets.QFileDialog.getExistingDirectory())
    #self.lineEdit.setText('{}'.format(directory))