from PySide6.QtCore import Qt, QSettings
from PySide6 import QtWidgets

# import the custom class made from QtDesigner
from ui_azure_config_common import Ui_Form
from azure_config_advanced import azureAdvancedSettingsWidget
from common_qt_functions import commonConfigFunctions, defaultSettingsManager

pipelineChoices = ['file_cache','stream']
bucketModeChoices = ["key", "sas", "spn", "msi"]
azStorageType = ["block", "adls"]
libfusePermissions = [0o777,0o666,0o644,0o444]

class azureSettingsWidget(defaultSettingsManager,commonConfigFunctions, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle("Azure Config Settings")
        self.myWindow = QSettings("LyveFUSE", "AzcWindow")
        self.initSettingsFromConfig()
        # Hide the pipeline mode groupbox depending on the default select is
        self.showAzureModeSettings()
        self.showModeSettings()
        self.populateOptions()

        # Set up signals
        self.dropDown_pipeline.currentIndexChanged.connect(self.showModeSettings)
        self.dropDown_azure_modeSetting.currentIndexChanged.connect(self.showAzureModeSettings)
        self.dropDown_azure_storageType.currentIndexChanged.connect(self.updateAzStorage)
        self.dropDown_azure_modeSetting.currentIndexChanged.connect(self.updateAzStorage)
        self.button_browse.clicked.connect(self.getFileDirInput)
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_advancedSettings.clicked.connect(self.openAdvanced)
        self.button_resetDefaultSettings.clicked.connect(self.resetDefaults)
    
    
        self.checkbox_commonConfig_multiUser.stateChanged.connect(self.updateMultiUser)
        self.checkbox_commonConfig_nonEmptyDir.stateChanged.connect(self.updateNonEmtpyDir)
        self.checkbox_daemonForeground.stateChanged.connect(self.updateDaemonForeground)
        self.checkbox_commonConfig_readOnly.stateChanged.connect(self.updateReadOnly)
        self.checkbox_libfuse_ignoreAppend.stateChanged.connect(self.updateLibfuse)
        self.checkbox_streaming_fileCachingLevel.stateChanged.connect(self.updateStream)
        
        self.spinBox_libfuse_attributeExp.valueChanged.connect(self.updateLibfuse)
        self.spinBox_libfuse_entryExp.valueChanged.connect(self.updateLibfuse)
        self.spinBox_libfuse_negEntryExp.valueChanged.connect(self.updateLibfuse)
        self.spinBox_streaming_blockSize.valueChanged.connect(self.updateStream)
        self.spinBox_streaming_buffSize.valueChanged.connect(self.updateStream)
        self.spinBox_streaming_maxBuff.valueChanged.connect(self.updateStream)
    
        self.lineEdit_azure_accountKey.setEchoMode(QtWidgets.QLineEdit.EchoMode.Password)
        self.lineEdit_azure_spnClientSecret.setEchoMode(QtWidgets.QLineEdit.EchoMode.Password)
        self.lineEdit_fileCache_path.editingFinished.connect(self.updateFileCache)
        self.lineEdit_azure_accountKey.editingFinished.connect(self.updateAzStorage)
        self.lineEdit_azure_sasStorage.editingFinished.connect(self.updateAzStorage)
        self.lineEdit_azure_accountName.editingFinished.connect(self.updateAzStorage)
        self.lineEdit_azure_container.editingFinished.connect(self.updateAzStorage)
        self.lineEdit_azure_endpoint.editingFinished.connect(self.updateAzStorage)
        self.lineEdit_azure_msiAppID.editingFinished.connect(self.updateAzStorage)
        self.lineEdit_azure_msiResourceID.editingFinished.connect(self.updateAzStorage)
        self.lineEdit_azure_msiObjectID.editingFinished.connect(self.updateAzStorage)
        self.lineEdit_azure_spnTenantID.editingFinished.connect(self.updateAzStorage)
        self.lineEdit_azure_spnClientID.editingFinished.connect(self.updateAzStorage)
        self.lineEdit_azure_spnClientSecret.editingFinished.connect(self.updateAzStorage)
    
    # Set up slots
    
    def updateMultiUser(self):
        self.settings.setValue('allow-other',self.checkbox_commonConfig_multiUser.isChecked())
        
    def updateNonEmtpyDir(self):
        self.settings.setValue('nonempty',self.checkbox_commonConfig_nonEmptyDir.isChecked())        
        
    def updateReadOnly(self):
        self.settings.setValue('read-only',self.checkbox_commonConfig_readOnly.isChecked())

    def updateDaemonForeground(self):
        self.settings.setValue('foreground',self.checkbox_daemonForeground.isChecked())    
    
    # Update Libfuse re-writes everything in the Libfuse because of how setting.setValue works - 
    #   it will not append, so the code makes a copy of the dictionary and updates the sub-keys. 
    #   When the user updates the sub-option through the GUI, it will trigger Libfuse to update;
    #   it's written this way to save on lines of code.
    def updateLibfuse(self):
        libfuse = self.settings.value('libfuse')
        libfuse['default-permission'] = libfusePermissions[self.dropDown_libfuse_permissions.currentIndex()]
        libfuse['ignore-open-flags'] = self.checkbox_libfuse_ignoreAppend.isChecked()
        libfuse['attribute-expiration-sec'] = self.spinBox_libfuse_attributeExp.value()
        libfuse['entry-expiration-sec'] = self.spinBox_libfuse_entryExp.value()
        libfuse['negative-entry-expiration-sec'] = self.spinBox_libfuse_negEntryExp.value()
        self.settings.setValue('libfuse',libfuse)
    
    
    # Update stream re-writes everything in the stream dictionary for the same reason update libfuse does.
    def updateStream(self):
        stream = self.settings.value('stream')
        stream['file-caching'] = self.checkbox_streaming_fileCachingLevel.isChecked()
        stream['block-size-mb'] = self.spinBox_streaming_blockSize.value()
        stream['buffer-size-mb'] = self.spinBox_streaming_buffSize.value()
        stream['max-buffers'] = self.spinBox_streaming_maxBuff.value()
        self.settings.setValue('stream',stream)
        
    def updateAzStorage(self):
        azStorage = self.settings.value('azstorage')
        azStorage['account-key'] = self.lineEdit_azure_accountKey.text()
        azStorage['sas'] = self.lineEdit_azure_sasStorage.text()
        azStorage['account-name'] = self.lineEdit_azure_accountName.text()
        azStorage['container'] = self.lineEdit_azure_container.text()
        azStorage['endpoint'] = self.lineEdit_azure_endpoint.text()
        azStorage['appid'] = self.lineEdit_azure_msiAppID.text()
        azStorage['resid'] = self.lineEdit_azure_msiResourceID.text()
        azStorage['objid'] = self.lineEdit_azure_msiObjectID.text()
        azStorage['tenantid'] = self.lineEdit_azure_spnTenantID.text()
        azStorage['clientid'] = self.lineEdit_azure_spnClientID.text()
        azStorage['clientsecret'] = self.lineEdit_azure_spnClientSecret.text()
        azStorage['type'] = azStorageType[self.dropDown_azure_storageType.currentIndex()]
        azStorage['mode'] = bucketModeChoices[self.dropDown_azure_modeSetting.currentIndex()]
        self.settings.setValue('azstorage',azStorage)
            
    def openAdvanced(self):
        self.moreSettings = azureAdvancedSettingsWidget()
        self.moreSettings.setWindowModality(Qt.ApplicationModal)
        self.moreSettings.show()

    # ShowModeSettings will switch which groupbox is visiible: stream or file_cache
    #   the function also updates the internal components settings through QSettings
    #   There is one slot for the signal to be pointed at which is why showmodesettings is used.
    def showModeSettings(self):
        self.hideModeBoxes()
        components = self.settings.value('components')
        pipelineIndex = self.dropDown_pipeline.currentIndex()
        components[1] = pipelineChoices[pipelineIndex]
        if pipelineChoices[pipelineIndex] == 'file_cache':
            self.groupbox_fileCache.setVisible(True)
        elif pipelineChoices[pipelineIndex] == 'stream':
            self.groupbox_streaming.setVisible(True)
        self.settings.setValue('components',components)
    
    def showAzureModeSettings(self):

        self.hideAzureBoxes()
        modeSelectionIndex = self.dropDown_azure_modeSetting.currentIndex()
        # Azure mode group boxes
        if bucketModeChoices[modeSelectionIndex] == "key":
            self.groupbox_accountKey.setVisible(True)
        elif bucketModeChoices[modeSelectionIndex] == "sas":
            self.groupbox_sasStorage.setVisible(True)
        elif bucketModeChoices[modeSelectionIndex] == "spn":
            self.groupbox_spn.setVisible(True)
        elif bucketModeChoices[modeSelectionIndex] == "msi":
            self.groupbox_msi.setVisible(True) 
            
# This widget will not display all the options in settings, only the ones written in the UI file.
    def populateOptions(self):
        # add commnet here
        self.dropDown_pipeline.setCurrentIndex(pipelineChoices.index(self.settings.value('components')[1]))
        self.dropDown_libfuse_permissions.setCurrentIndex(libfusePermissions.index(self.settings.value('libfuse')['default-permission']))
        self.dropDown_azure_storageType.setCurrentIndex(azStorageType.index(self.settings.value('azstorage')['type']))
        self.dropDown_azure_modeSetting.setCurrentIndex(bucketModeChoices.index(self.settings.value('azstorage')['mode']))
        
        # Check for a true/false setting and set the checkbox state as appropriate. 
        #   Note, Checked/UnChecked are NOT True/False data types, hence the need to check what the values are.
        #   The default values for True/False settings are False, which is why Unchecked is the default state.
        if self.settings.value('allow-other') == True:
            self.checkbox_commonConfig_multiUser.setCheckState(Qt.Checked)
        else:
            self.checkbox_commonConfig_multiUser.setCheckState(Qt.Unchecked)
        
        if self.settings.value('nonempty') == True:
            self.checkbox_commonConfig_nonEmptyDir.setCheckState(Qt.Checked)
        else:
            self.checkbox_commonConfig_nonEmptyDir.setCheckState(Qt.Unchecked)            

        if self.settings.value('foreground') == True:
            self.checkbox_daemonForeground.setCheckState(Qt.Checked)
        else:
            self.checkbox_daemonForeground.setCheckState(Qt.Unchecked)  
        
        if self.settings.value('read-only') == True:
            self.checkbox_commonConfig_readOnly.setCheckState(Qt.Checked)
        else:
            self.checkbox_commonConfig_readOnly.setCheckState(Qt.Unchecked)    

        if self.settings.value('stream')['file-caching'] == True:
            self.checkbox_streaming_fileCachingLevel.setCheckState(Qt.Checked)
        else:
            self.checkbox_streaming_fileCachingLevel.setCheckState(Qt.Unchecked)
            
        if self.settings.value('libfuse')['ignore-open-flags'] == True:
            self.checkbox_streaming_fileCachingLevel.setCheckState(Qt.Checked)
        else:
            self.checkbox_streaming_fileCachingLevel.setCheckState(Qt.Unchecked)
        
        # Spinbox automatically sanitizes intputs for decimal values only, so no need to check for the appropriate data type.
        self.spinBox_libfuse_attributeExp.setValue(self.settings.value('libfuse')['attribute-expiration-sec'])
        self.spinBox_libfuse_entryExp.setValue(self.settings.value('libfuse')['entry-expiration-sec'])
        self.spinBox_libfuse_negEntryExp.setValue(self.settings.value('libfuse')['negative-entry-expiration-sec'])
        self.spinBox_streaming_blockSize.setValue(self.settings.value('stream')['block-size-mb'])
        self.spinBox_streaming_buffSize.setValue(self.settings.value('stream')['buffer-size-mb'])
        self.spinBox_streaming_maxBuff.setValue(self.settings.value('stream')['max-buffers'])
        
        # There is no sanitizing for lineEdit at the moment, the GUI depends on the user being correc.

        self.lineEdit_azure_accountKey.setText(self.settings.value('azstorage')['account-key'])
        self.lineEdit_azure_sasStorage.setText(self.settings.value('azstorage')['sas'])
        self.lineEdit_azure_accountName.setText(self.settings.value('azstorage')['account-name'])
        self.lineEdit_azure_container.setText(self.settings.value('azstorage')['container'])
        self.lineEdit_azure_endpoint.setText(self.settings.value('azstorage')['endpoint'])
        self.lineEdit_azure_msiAppID.setText(self.settings.value('azstorage')['appid'])
        self.lineEdit_azure_msiResourceID.setText(self.settings.value('azstorage')['resid'])
        self.lineEdit_azure_msiObjectID.setText(self.settings.value('azstorage')['objid'])
        self.lineEdit_azure_spnTenantID.setText(self.settings.value('azstorage')['tenantid'])
        self.lineEdit_azure_spnClientID.setText(self.settings.value('azstorage')['clientid'])
        self.lineEdit_azure_spnClientSecret.setText(self.settings.value('azstorage')['clientsecret'])
        self.lineEdit_fileCache_path.setText(self.settings.value('file_cache')['path'])
    
    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.lineEdit_fileCache_path.setText('{}'.format(directory))
        # Update the settings 
        self.updateFileCache()
       
    def updateFileCache(self):
        filePath = self.settings.value('file_cache')
        filePath['path'] = self.lineEdit_fileCache_path.text()
        self.settings.setValue('file_cache',filePath)
 
    def hideModeBoxes(self):
        self.groupbox_fileCache.setVisible(False)
        self.groupbox_streaming.setVisible(False)
        
        
    def hideAzureBoxes(self):
        self.groupbox_accountKey.setVisible(False)
        self.groupbox_sasStorage.setVisible(False)
        self.groupbox_spn.setVisible(False)
        self.groupbox_msi.setVisible(False)

    def resetDefaults(self):
        pass