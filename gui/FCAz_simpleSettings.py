from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget

from ui_FCAz_simpleSettings import Ui_FCAz_simpleSettings

class fcazSettingsWidget(QWidget, Ui_FCAz_simpleSettings):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        # Set the title of the widget window
        self.setWindowTitle("Settings")