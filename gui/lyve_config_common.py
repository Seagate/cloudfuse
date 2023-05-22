from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets
import yaml
import os

# import the custom class made from QtDesigner
from ui_lyve_config_common import Ui_Form
from lyve_config_advanced import lyveAdvancedSettingsWidget
from common_qt_functions import closeGUIEvent, settingsManager

pipelineChoices = {
    "fileCache" : 0,
    "streaming" : 1
}

libfusePermissions = {
    0o777 : 0,
    0o666 : 1,
    0o644 : 2,
    0o444 : 3
}

class lyveSettingsWidget(settingsManager,closeGUIEvent,Ui_Form): 
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        self.setWindowTitle("LyveCloud Config Settings")
        self.initSettingsFromConfig()
        self.populateOptions()
        self.showModeSettings()
                
        # Hide sensitive data QtWidgets.QLineEdit.EchoMode.PasswordEchoOnEdit
        self.lineEdit_accessKey.setEchoMode(QtWidgets.QLineEdit.EchoMode.Password) 
        self.lineEdit_secretKey.setEchoMode(QtWidgets.QLineEdit.EchoMode.Password)

        # Set up signals for buttons
        self.dropDown_pipeline.currentIndexChanged.connect(self.showModeSettings)
        self.button_browse.clicked.connect(self.getFileDirInput)
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_advancedSettings.clicked.connect(self.openAdvanced)
        
        try:
            self.resize(self.settings.value("lyc window size"))
            self.move(self.settings.value("lyc window position"))
        except:
            pass

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
        
    def initSettingsFromConfig(self):
        dictForConfigs = self.getConfigs()
        for option in dictForConfigs:
            if type(dictForConfigs[option]) == dict:
                tempDict = self.settings.value(option)
                for suboption in dictForConfigs[option]:
                    tempDict[suboption] = dictForConfigs[option][suboption]
                self.settings.setValue(option,tempDict)
            else:
                self.settings.setValue(option,dictForConfigs[option])
        return

    def showOptionStatesFromSettings(self):
        pass

    def populateOptions(self):
        if self.settings.value('components')[1] == 'file_cache':
            self.dropDown_pipeline.setCurrentIndex(pipelineChoices['fileCache'])
        else:
            self.dropDown_pipeline.setCurrentIndex(pipelineChoices['streaming'])
        
        self.dropDown_libfuse_permissions.setCurrentIndex(libfusePermissions[self.settings.value('libfuse')['default-permission']])
        
        if self.settings.value('allow-other') == False:
            self.checkbox_multiUser.setCheckState(Qt.Unchecked)
        else:
            self.checkbox_multiUser.setCheckState(Qt.Checked)
        
        if self.settings.value('nonempty') == False:
            self.checkbox_nonEmptyDir.setCheckState(Qt.Unchecked)
        else:
            self.checkbox_nonEmptyDir.setCheckState(Qt.Checked)            

        if self.settings.value('foreground') == False:
            self.checkbox_daemonForeground.setCheckState(Qt.Unchecked)
        else:
            self.checkbox_daemonForeground.setCheckState(Qt.Checked)  
        
        if self.settings.value('read-only') == False:
            self.checkbox_readOnly.setCheckState(Qt.Unchecked)
        else:
            self.checkbox_readOnly.setCheckState(Qt.Checked)    

        if self.settings.value('stream')['file-caching'] == False:
            self.checkbox_streaming_fileCachingLevel.setCheckState(Qt.Unchecked)
        else:
            self.checkBox_streaming_fileCachingLevel.setCheckState(Qt.Checked)
        
        return

    def writeConfigFile(self):
        dictForConfigs = self.getConfigs()
        currentDir = os.getcwd()
        
   
        if self.dropDown_pipeline.currentIndex() == pipelineChoices['fileCache']:
            dictForConfigs['components'][1] = 'file_cache'
        elif self.dropDown_pipeline.currentIndex() == pipelineChoices['streaming']:
            dictForConfigs['components'][1] = 'stream'
        
        dictForConfigs['allow-other']=self.checkbox_multiUser.isChecked()
        dictForConfigs['nonempty']=self.checkbox_nonEmptyDir.isChecked()
        dictForConfigs['foreground']=self.checkbox_daemonForeground.isChecked()
        dictForConfigs['read-only']=self.checkbox_readOnly.isChecked()
       
        # Write libfuse components
        dictForConfigs['libfuse']['ignore-open-flags'] = self.checkbox_libfuse_ignoreAppend.isChecked()

        ###### fix this ##### -- get it to write 0444 NOT 0o444, yaml interprets 0o444 as a string, not a number
        tempOption = oct(int(self.dropDown_libfuse_permissions.currentText(),8))
        dictForConfigs['libfuse']['default-permission'] = tempOption
        
        index = self.dropDown_libfuse_permissions.currentIndex()
        if index == libfusePermissions['0o777']:
            dictForConfigs['libfuse']['default-permission'] = 0o777
        elif index == libfusePermissions['0o666']:
            dictForConfigs['libfuse']['default-permission'] = 0o666
        elif index == libfusePermissions['0o644']:
            dictForConfigs['libfuse']['default-permission'] = 0o644
        elif index == libfusePermissions['0o444']:
            dictForConfigs['libfuse']['default-permission'] = 0o444
            
        
        
        dictForConfigs['libfuse']['attribute-expiration-sec'] = self.spinBox_libfuse_attExp.value()
        dictForConfigs['libfuse']['entry-expiration-sec'] = self.spinBox_libfuse_entExp.value()
        dictForConfigs['libfuse']['negative-entry-expiration-sec'] = self.spinBox_libfuse_negEntryExp.value()
        
        # Write S3 Storage
        dictForConfigs['s3storage']['secret-key'] = self.lineEdit_secretKey.text()
        dictForConfigs['s3storage']['endpoint'] = self.lineEdit_endpoint.text()
        dictForConfigs['s3storage']['key-id'] = self.lineEdit_accessKey.text()
        dictForConfigs['s3storage']['bucket-name'] = self.lineEdit_bucketName.text()
        
        # File cache folder path
        dictForConfigs['file_cache']['path'] = self.lineEdit_fileCache_path.text()

        with open(currentDir+'/testing_config.yaml','w') as file:
            yaml.safe_dump(dictForConfigs,file)
        
        return
    
    def getConfigs(self,useDefault=False):
        currentDir = os.getcwd()
        if useDefault:
                with open(currentDir+'/default_config.yaml','r') as file:
                    configs = yaml.safe_load(file)
        else:
            try:
                with open(currentDir+'/config.yaml', 'r') as file:
                    configs = yaml.safe_load(file,)
            except:
                configs = self.getConfigs(True)
        return configs
    
    def exitWindow(self):
        self.settings.setValue("lyc window size", self.size())
        self.settings.setValue("lyc window position", self.pos())
        self.close()