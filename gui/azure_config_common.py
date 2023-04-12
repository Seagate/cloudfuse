from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets
import yaml

# import the custom class made from QtDesigner
from ui_azure_config_common import Ui_Form
from azure_config_advanced import azureAdvancedSettingsWidget
from common_qt_functions import closeGUIEvent

pipelineChoices = {
    "fileCache" : 0,
    "streaming" : 1
}

bucketModeChoices = {
    "key" : 0,
    "sas" : 1,
    "spn" : 2,
    "msi" : 3
}

class azureSettingsWidget(closeGUIEvent, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)

        self.setWindowTitle("Azure Config Settings")

        # Hide the pipeline mode groupbox depending on the default select is
        self.showAzureModeSettings()
        self.showModeSettings()

        # Set up signals
        self.dropDown_pipeline.currentIndexChanged.connect(self.showModeSettings)
        self.dropDown_azure_modeSetting.currentIndexChanged.connect(self.showAzureModeSettings)
        self.button_browse.clicked.connect(self.getFileDirInput)
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_advancedSettings.clicked.connect(self.openAdvanced)
    
        self.lineEdit_azure_accountKey.setEchoMode(QtWidgets.QLineEdit.EchoMode.Password)
        self.lineEdit_azure_spnClientSecret.setEchoMode(QtWidgets.QLineEdit.EchoMode.Password)
    
    
    # Set up slots
    def openAdvanced(self):
        self.moreSettings = azureAdvancedSettingsWidget()
        self.moreSettings.setWindowModality(Qt.ApplicationModal)
        self.moreSettings.show()



    def showModeSettings(self):
        
        self.hideModeBoxes()
        
        pipelineSelection = self.dropDown_pipeline.currentIndex()
        
        if pipelineSelection == pipelineChoices['fileCache']:
            self.groupbox_fileCache.setVisible(True)
        elif pipelineSelection == pipelineChoices['streaming']:
            self.groupbox_streaming.setVisible(True)
            

    def showAzureModeSettings(self):

        self.hideAzureBoxes()
        modeSelection = self.dropDown_azure_modeSetting.currentIndex()

        # Azure mode group boxes
        if modeSelection == bucketModeChoices["key"]:
            self.groupbox_accountKey.setVisible(True)
        elif modeSelection == bucketModeChoices["sas"]:
            self.groupbox_sasStorage.setVisible(True)
        elif modeSelection == bucketModeChoices["spn"]:
            self.groupbox_spn.setVisible(True)
        elif modeSelection == bucketModeChoices["msi"]:
            self.groupbox_msi.setVisible(True) 
    
    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.lineEdit_fileCache_path.setText('{}'.format(directory))
        
    def hideModeBoxes(self):
        self.groupbox_fileCache.setVisible(False)
        self.groupbox_streaming.setVisible(False)
        
        
    def hideAzureBoxes(self):
        self.groupbox_accountKey.setVisible(False)
        self.groupbox_sasStorage.setVisible(False)
        self.groupbox_spn.setVisible(False)
        self.groupbox_msi.setVisible(False)
        

    def writeConfigFile(self):
        # Add relevant code here
        return
        
    def exitWindow(self):
        self.close()
