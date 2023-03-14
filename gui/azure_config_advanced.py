from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets
# import the custom class made from QtDesigner
from ui_azure_config_advanced import Ui_Form

class azureAdvancedSettingsWidget(QWidget, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle("More Azure Config Settings")
        
        # Set up the signals
        self.button_okay.clicked.connect(self.exitWindow)
        
        
        
        
    def exitWindow(self):
        self.close()

    def closeEvent(self, event):
               
        # Double check with user before closing
        
        msg = QtWidgets.QMessageBox()
        msg.setWindowTitle("Are you sure?")
        msg.setText("You have clicked the okay button.")
        msg.setInformativeText("Do you want to save you changes?")
        msg.setStandardButtons(QtWidgets.QMessageBox.Save | QtWidgets.QMessageBox.Discard | QtWidgets.QMessageBox.Cancel)
        msg.setDefaultButton(QtWidgets.QMessageBox.Save)
        ret = msg.exec()        

        # Insert all settings to yaml file
        
        event.accept()