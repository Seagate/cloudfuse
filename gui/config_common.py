from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget

# import the custom class made from QtDesigner
from ui_config_common import Ui_Form

pipeline = {
    "streaming" : 1,
    "filecaching" : 0
}

class mountSettingsWidget(QWidget, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)


    if self.pipeline_select.currentIndex() == pipeline["filecachine"]:
        self.blockSize_label.vis