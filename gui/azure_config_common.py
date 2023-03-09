from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets
# import the custom class made from QtDesigner
from ui_azure_config_common import Ui_Form

from azure_config_advanced import azureAdvancedSettingsWidget

class azureSettingsWidget(QWidget, Ui_Form):
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
    
    
    
    # Set up slots
    def openAdvanced(self):
        self.moreSettings = azureAdvancedSettingsWidget()
        self.moreSettings.show()



    def showModeSettings(self):
        
        self.hideModeBoxes()
        
        match self.dropDown_pipeline.currentIndex():
            case 0:
                self.groupbox_fileCache.setVisible(True)
            case 1:
                self.groupbox_streaming.setVisible(True)
            

    def showAzureModeSettings(self):

        self.hideAzureBoxes()
        
        # Azure mode group boxes
        match self.dropDown_azure_modeSetting.currentIndex():
            case 0:
                self.groupbox_accountKey.setVisible(True)
            case 1:
                self.groupbox_sasStorage.setVisible(True)
            case 2:
                self.groupbox_spn.setVisible(True)
            case 3:
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