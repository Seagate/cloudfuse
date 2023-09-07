from PySide6.QtWidgets import QWidget


from ui_about import Ui_Form
from ui_about_dialog import Ui_About
#from ui_s3_config_common import Ui_Form

class aboutPage(QWidget, Ui_About):
    def __init__(self):
        super().__init__()
        self.setupUi(self)