from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget

from ui_StL_simpleSettings import Ui_StL_simpleSettings

class stlzSettingsWidget(QWidget, Ui_StL_simpleSettings):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        # Set the title of the widget window
        self.setWindowTitle("Settings")