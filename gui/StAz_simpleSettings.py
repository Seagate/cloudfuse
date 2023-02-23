from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget

from ui_StAz_simpleSettings import Ui_StAz_simpleSettings

class stazSettingsWidget(QWidget, Ui_StAz_simpleSettings):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        # Set the title of the widget window
        self.setWindowTitle("Settings")