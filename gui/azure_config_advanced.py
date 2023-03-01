from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets
# import the custom class made from QtDesigner
from ui_azure_config_advanced import Ui_Form

class azureAdvancedSettingsWidget(QWidget, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle("More LyveCloud Config Settings")
        
        # Set up the signals
        self.okay_button.clicked.connect(self.exitWindow)
        
        
        
        
    def exitWindow(self):
        self.close()

    def closeEvent(self, event):
               
        # Insert all settings to yaml file
        if self.azAadEndpoint_input.text() == '':
            print('Are you sure?')
        
        # TODO: Double check with user before closing
        event.accept()