from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets
# import the custom class made from QtDesigner
from ui_config_common import Ui_Form

class lyveSettingsWidget(QWidget, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        self.setWindowTitle("LyveCloud Config Settings")
        self.showModeSettings()
       
        self.pipeline_select.currentIndexChanged.connect(self.showModeSettings)
        self.browse_button.clicked.connect(self.getFileDirInput)
        self.okay_button.clicked.connect(self.exitWindow)

    def showModeSettings(self):
        
        self.hideModeBoxes()
       
        match self.pipeline_select.currentIndex():
            case 0:
                self.filecache_groupbox.setVisible(True)
            case 1:
                self.streaming_groupbox.setVisible(True)

            
    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.fileCache_path_input.setText('{}'.format(directory))
        
    def hideModeBoxes(self):
        self.filecache_groupbox.setVisible(False)
        self.streaming_groupbox.setVisible(False)        
        
    def exitWindow(self):
        self.close()

    def closeEvent(self, event):
               
        # Insert all settings to yaml file
        if self.fileCache_path_input.text() == '' and self.pipeline_select.currentIndex() == 0:
            print('Are you sure?')
        
        # TODO: Double check with user before closing
        event.accept()