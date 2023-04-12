from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets
# import the custom class made from QtDesigner
from ui_lyve_config_advanced import Ui_Form
from closeGUIEvent import closeGUIEvent

class lyveAdvancedSettingsWidget(closeGUIEvent, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle("More LyveCloud Config Settings")
        
        # Set up the signals
        self.button_okay.clicked.connect(self.exitWindow)
        
        
    def exitWindow(self):
        self.close()

    def writeConfigFile(self):
        return