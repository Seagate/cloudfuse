from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget

from ui_FCL_simpleSettings import Ui_FCL_simpleSettings

class fclSettingsWidget(QWidget, Ui_FCL_simpleSettings):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        # Set the title of the widget window
        self.setWindowTitle("Settings")