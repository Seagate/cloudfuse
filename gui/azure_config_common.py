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

        self.setWindowTitle("LyveCloud Config Settings")

        # Hide the pipeline mode groupbox depending on the default select is
        self.showAzureModeSettings()
        self.showModeSettings()

        # Set up signals
        self.pipeline_select.currentIndexChanged.connect(self.showModeSettings)
        self.azModesettings_select.currentIndexChanged.connect(self.showAzureModeSettings)
        self.browse_button.clicked.connect(self.getFileDirInput)
        self.okay_button.clicked.connect(self.exitWindow)
        self.advanced_settings.clicked.connect(self.openAdvanced)
    
    
    
    # Set up slots
    def openAdvanced(self):
        self.moreSettings = azureAdvancedSettingsWidget()
        self.moreSettings.show()



    def showModeSettings(self):
        
        self.hideModeBoxes()
        
        match self.pipeline_select.currentIndex():
            case 0:
                self.filecache_groupbox.setVisible(True)
            case 1:
                self.streaming_groupbox.setVisible(True)
            

    def showAzureModeSettings(self):

        self.hideAzureBoxes()
        
        # Azure mode group boxes
        match self.azModesettings_select.currentIndex():
            case 0:
                self.accnt_key_groupbox.setVisible(True)
            case 1:
                self.sas_storage_groupbox.setVisible(True)
            case 2:
                self.spn_groupbox.setVisible(True)
            case 3:
                self.msi_groupbox.setVisible(True)  
        
    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.fileCache_path_input.setText('{}'.format(directory))
        
    def hideModeBoxes(self):
        self.filecache_groupbox.setVisible(False)
        self.streaming_groupbox.setVisible(False)
        
        
    def hideAzureBoxes(self):
        self.accnt_key_groupbox.setVisible(False)
        self.sas_storage_groupbox.setVisible(False)
        self.spn_groupbox.setVisible(False)
        self.msi_groupbox.setVisible(False)
        
        
    def exitWindow(self):
        self.close()

    def closeEvent(self, event):
               
        # Insert all settings to yaml file
        if self.azAccountName_input.text() == '':
            print('Are you sure?')
        
        # TODO: Double check with user before closing
        event.accept()