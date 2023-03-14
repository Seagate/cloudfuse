from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets

# import the custom class made from QtDesigner
from ui_lyve_config_common import Ui_Form
from lyve_config_advanced import lyveAdvancedSettingsWidget

class lyveSettingsWidget(QWidget, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        self.setWindowTitle("LyveCloud Config Settings")
        self.showModeSettings()
       
        self.dropDown_pipeline.currentIndexChanged.connect(self.showModeSettings)
        self.button_browse.clicked.connect(self.getFileDirInput)
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_advancedSettings.clicked.connect(self.openAdvanced)

    # Set up slots
    def openAdvanced(self):
        self.moreSettings = lyveAdvancedSettingsWidget()
        self.moreSettings.show()

    def showModeSettings(self):
        
        self.hideModeBoxes()
       
        match self.dropDown_pipeline.currentIndex():
            case 0:
                self.groupbox_fileCache.setVisible(True)
            case 1:
                self.groupbox_streaming.setVisible(True)

            
    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.lineEdit_fileCache_path.setText('{}'.format(directory))
        
    def hideModeBoxes(self):
        self.groupbox_fileCache.setVisible(False)
        self.groupbox_streaming.setVisible(False)        
        
    def exitWindow(self):
        self.close()

    def closeEvent(self, event):
        
        # Double check with user before closing
        
        msg = QtWidgets.QMessageBox()
        msg.setWindowTitle("Are you sure?")
        msg.setText("You have clicked the okay button.")
        msg.setInformativeText("Do you want to save you changes?")
        msg.setStandardButtons(QtWidgets.QMessageBox.Save | QtWidgets.QMessageBox.Discard | QtWidgets.QMessageBox.Cancel)
        msg.setDefaultButton(QtWidgets.QMessageBox.Save)
        ret = msg.exec()
               
        # Insert all settings to yaml file

        event.accept()