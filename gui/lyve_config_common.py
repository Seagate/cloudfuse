from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets
import yaml

# import the custom class made from QtDesigner
from ui_lyve_config_common import Ui_Form
from lyve_config_advanced import lyveAdvancedSettingsWidget
from common_qt_functions import closeGUIEvent

pipelineChoices = {
    "fileCache" : 0,
    "streaming" : 1
}

class lyveSettingsWidget(closeGUIEvent, Ui_Form): 
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        self.setWindowTitle("LyveCloud Config Settings")
        self.showModeSettings()
       
        # Hide sensitive data QtWidgets.QLineEdit.EchoMode.PasswordEchoOnEdit
        self.lineEdit_accessKey.setEchoMode(QtWidgets.QLineEdit.EchoMode.Password) 
        self.lineEdit_secretKey.setEchoMode(QtWidgets.QLineEdit.EchoMode.Password)

        # Set up signals for buttons
        self.dropDown_pipeline.currentIndexChanged.connect(self.showModeSettings)
        self.button_browse.clicked.connect(self.getFileDirInput)
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_advancedSettings.clicked.connect(self.openAdvanced)

        

    # Set up slots
    def openAdvanced(self):
        self.moreSettings = lyveAdvancedSettingsWidget()
        self.moreSettings.setWindowModality(Qt.ApplicationModal)
        self.moreSettings.show()

    def showModeSettings(self):
        
        self.hideModeBoxes()
        
        pipelineSelection = self.dropDown_pipeline.currentIndex()
        
        if pipelineSelection == pipelineChoices['fileCache']:
            self.groupbox_fileCache.setVisible(True)
        elif pipelineSelection == pipelineChoices['streaming']:
            self.groupbox_streaming.setVisible(True)
            
    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.lineEdit_fileCache_path.setText('{}'.format(directory))
        
    def hideModeBoxes(self):
        self.groupbox_fileCache.setVisible(False)
        self.groupbox_streaming.setVisible(False)        
        
    def exitWindow(self):
        self.close()

    def initOptionsFromConfig(self):
        return

    def writeConfigFile(self):
         configs = self.getConfigs()
        # print(configs)
        return
    
    def getConfigs(self,useDefault=False):
        if useDefault:
            with open('/home/tinker/code/lyvecloudfuse/default_config.yaml','r') as file:
                configs = yaml.safe_load(file)
        else:
            with open('/home/tinker/code/lyvecloudfuse/config.yaml', 'r') as file:
                configs = yaml.safe_load(file)
        return configs