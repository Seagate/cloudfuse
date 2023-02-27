from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget

# import the custom class made from QtDesigner
from ui_config_common import Ui_Form

class lyveSettingsWidget(QWidget, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)

        if self.pipeline_select.currentIndex() == 0:
            # self.groupBox2.setVisible(False)
            self.streaming_groupbox.setVisible(False)
        else:
            self.filecache_groupbox.setVisible(False)
        
        self.pipeline_select.currentIndexChanged.connect(self.showModeSettings)


    def showModeSettings(self):
        if self.pipeline_select.currentIndex() == 0:
            self.streaming_groupbox.setVisible(False)
            self.filecache_groupbox.setVisible(True)
        else:
            self.streaming_groupbox.setVisible(True)
            self.filecache_groupbox.setVisible(False)