from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets
import yaml
import os

# import the custom class made from QtDesigner
from ui_lyve_config_common import Ui_Form
from lyve_config_advanced import lyveAdvancedSettingsWidget
from common_qt_functions import closeGUIEvent

pipelineChoices = {
    "fileCache" : 0,
    "streaming" : 1
}

libfusePermissions = {
    '0o777' : 0,
    '0o666' : 1,
    '0o644' : 2,
    '0o444' : 3
}

dictForConfigs = []

class lyveSettingsWidget(closeGUIEvent, Ui_Form): 
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        self.setWindowTitle("LyveCloud Config Settings")
        self.initOptionsFromConfig()
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
        
    def initOptionsFromConfig(self):
        dictForConfigs = self.getConfigs()
        
        if dictForConfigs.get('allow-other') == True:
            self.checkbox_multiUser.setCheckState(Qt.CheckState.Checked)
        elif dictForConfigs.get('allow-other') == False:
            self.checkbox_multiUser.setCheckState(Qt.CheckState.Unchecked)

        if dictForConfigs.get('nonempty') == True:
            self.checkbox_nonEmptyDir.setCheckState(Qt.CheckState.Checked)
        elif dictForConfigs.get('nonempty') == False:
            self.checkbox_nonEmptyDir.setCheckState(Qt.CheckState.Unchecked)

        if dictForConfigs.get('foreground') == True:
            self.checkbox_daemonForeground.setCheckState(Qt.CheckState.Checked)
        elif dictForConfigs.get('foreground') == False:
            self.checkbox_daemonForeground.setCheckState(Qt.CheckState.Unchecked)
            
        if dictForConfigs.get('libfuse').get('ignore-open-flags') == True:
            self.checkbox_libfuse_ignoreAppend.setCheckState(Qt.CheckState.Checked)
        elif dictForConfigs.get('libfuse').get('ignore-open-flags') == False:
            self.checkbox_libfuse_ignoreAppend.setCheckState(Qt.CheckState.Unchecked)
        
        if dictForConfigs.get('libfuse').get('default-permission') != None:
            self.dropDown_libfuse_permissions.setCurrentIndex(libfusePermissions[oct(dictForConfigs.get('libfuse').get('default-permission'))])

        if dictForConfigs.get('libfuse').get('attribute-expiration-sec') != None:
            self.spinBox_libfuse_attExp.setValue(int(dictForConfigs.get('libfuse').get('attribute-expiration-sec')))

        if dictForConfigs.get('libfuse').get('entry-expiration-sec') != None:
            self.spinBox_libfuse_entExp.setValue(int(dictForConfigs.get('libfuse').get('entry-expiration-sec')))
            
        if dictForConfigs.get('libfuse').get('negative-entry-expiration-sec') != None:
            self.spinBox_libfuse_negEntryExp.setValue(int(dictForConfigs.get('libfuse').get('negative-entry-expiration-sec')))

        if dictForConfigs.get('read-only') == True:
            self.checkbox_readOnly.setCheckState(Qt.CheckState.Checked)
        elif dictForConfigs.get('read-only') == False:
            self.checkbox_readOnly.setCheckState(Qt.CheckState.Unchecked)
                       
        if 'file_cache' in dictForConfigs.get('components'):
            self.dropDown_pipeline.setCurrentIndex(pipelineChoices['fileCache'])
        elif 'stream' in dictForConfigs.get('components'):
            self.dropDown_pipeline.setCurrentIndex(pipelineChoices['streaming'])
            
        
            
        
        
        # if dictForConfigs.get('') == True:
        # elif dictForConfigs.get('') == False:
            
        # if dictForConfigs.get('') == True:
        # elif dictForConfigs.get('') == False:            
        # self.checkbox_multiUser.setCheckState(dictForConfigs.get('allow-other'))
        # self.checkbox_nonEmptyDir.setCheckState(dictForConfigs.get('nonempty'))
        

        
        
        return

    def writeConfigFile(self):
        configs = self.getConfigs()

        # print(configs)
        return
    
    def getConfigs(self,useDefault=False):
        currentDir = os.getcwd()
        if useDefault:
                with open(currentDir+'/default_config.yaml','r') as file:
                    configs = yaml.safe_load(file)
        else:
            try:
                with open(currentDir+'/config.yaml', 'r') as file:
                    configs = yaml.safe_load(file)
            except:
                configs = self.getConfigs(True)
        return configs