from PySide6.QtCore import Qt, QSettings
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets

# import the custom class made from QtDesigner
from ui_lyve_config_common import Ui_Form
from lyve_config_advanced import lyveAdvancedSettingsWidget
from common_qt_functions import closeGUIEvent, defaultSettingsManager,commonConfigFunctions

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

class lyveSettingsWidget(defaultSettingsManager,closeGUIEvent,commonConfigFunctions,Ui_Form): 
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        self.initWindowSizePos()
        self.setWindowTitle("LyveCloud Config Settings")
        self.initSettingsFromConfig()
        self.populateOptions()
        self.showModeSettings()

        # Hide sensitive data QtWidgets.QLineEdit.EchoMode.PasswordEchoOnEdit
        self.lineEdit_accessKey.setEchoMode(QtWidgets.QLineEdit.EchoMode.Password)
        self.lineEdit_secretKey.setEchoMode(QtWidgets.QLineEdit.EchoMode.Password)

        # Set up signals for buttons
        self.dropDown_pipeline.currentIndexChanged.connect(self.showModeSettings)
        self.dropDown_libfuse_permissions.currentIndexChanged.connect(self.updateLibfuse)
        
        self.button_browse.clicked.connect(self.getFileDirInput)
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_advancedSettings.clicked.connect(self.openAdvanced)
        
        self.checkbox_multiUser.stateChanged.connect(self.updateMultiUser)
        self.checkbox_nonEmptyDir.stateChanged.connect(self.updateNonEmtpyDir)
        self.checkbox_daemonForeground.stateChanged.connect(self.updateDaemonForeground)
        self.checkbox_readOnly.stateChanged.connect(self.updateReadOnly)
        self.checkbox_libfuse_ignoreAppend.stateChanged.connect(self.updateLibfuse)
        self.checkbox_streaming_fileCachingLevel.stateChanged.connect(self.updateStream)
        
        self.spinBox_libfuse_attExp.valueChanged.connect(self.updateLibfuse)
        self.spinBox_libfuse_entExp.valueChanged.connect(self.updateLibfuse)
        self.spinBox_libfuse_negEntryExp.valueChanged.connect(self.updateLibfuse)
        self.spinBox_streaming_blockSize.valueChanged.connect(self.updateStream)
        self.spinBox_streaming_buffSize.valueChanged.connect(self.updateStream)
        self.spinBox_streaming_maxBuff.valueChanged.connect(self.updateStream)
        
        self.lineEdit_secretKey.editingFinished.connect(self.updateS3Storage)
        self.lineEdit_bucketName.editingFinished.connect(self.updateS3Storage)
        self.lineEdit_accessKey.editingFinished.connect(self.updateS3Storage)
        self.lineEdit_endpoint.editingFinished.connect(self.updateS3Storage)

       
    # Set up slots
    
    def updateMultiUser(self):
        self.settings.setValue('allow-other',self.checkbox_multiUser.isChecked())
        
    def updateNonEmtpyDir(self):
        self.settings.setValue('nonempty',self.checkbox_nonEmptyDir.isChecked())        
    
    def updateDaemonForeground(self):
        self.settings.setValue('foreground',self.checkbox_daemonForeground.isChecked())
    
    def updateReadOnly(self):
        self.settings.setValue('read-only',self.checkbox_readOnly.isChecked())
    
    def updateLibfuse(self):
        libfuse = self.settings.value('libfuse')
        
        permissionState = self.dropDown_libfuse_permissions.currentIndex()
        
        if permissionState == libfusePermissions[0o777]:
            libfuse['default-permission'] = 0o777
        elif permissionState == libfusePermissions[0o666]:
            libfuse['default-permission'] = 0o666
        elif permissionState == libfusePermissions[0o644]:
            libfuse['default-permission'] = 0o644
        elif permissionState == libfusePermissions[0o444]:
            libfuse['default-permission'] = 0o444
            
        libfuse['ignore-open-flags'] = self.checkbox_libfuse_ignoreAppend.isChecked()
        libfuse['attribute-expiration-sec'] = self.spinBox_libfuse_attExp.value()
        libfuse['entry-expiration-sec'] = self.spinBox_libfuse_entExp.value()
        libfuse['negative-entry-expiration-sec'] = self.spinBox_libfuse_negEntryExp.value()
        self.settings.setValue('libfuse',libfuse)

    def updateStream(self):
        stream = self.settings.value('stream')
        stream['file-caching'] = self.checkbox_streaming_fileCachingLevel.isChecked()
        stream['block-size-mb'] = self.spinBox_streaming_blockSize.value()
        stream['buffer-size-mb'] = self.spinBox_streaming_buffSize.value()
        stream['max-buffers'] = self.spinBox_streaming_maxBuff.value()
        self.settings.setValue('stream',stream)
        
    def updateS3Storage(self):
        s3Storage = self.settings.value('s3storage')
        s3Storage['bucket-name'] = self.lineEdit_bucketName.text()
        s3Storage['key-id'] = self.lineEdit_accessKey.text()
        s3Storage['secret-key'] = self.lineEdit_secretKey.text()
        s3Storage['endpoint'] = self.lineEdit_endpoint.text()
        self.settings.setValue('s3storage',s3Storage)
        
    
    def openAdvanced(self):
        self.moreSettings = lyveAdvancedSettingsWidget()
        self.moreSettings.setWindowModality(Qt.ApplicationModal)
        self.moreSettings.show()

    def showModeSettings(self):
        self.hideModeBoxes()
        pipelineSelection = self.dropDown_pipeline.currentIndex()
        components = self.settings.value('components')
        if pipelineSelection == pipelineChoices['fileCache']:
            components[1] = 'file_cache'
            self.groupbox_fileCache.setVisible(True)
        elif pipelineSelection == pipelineChoices['streaming']:
            components[1] = 'stream'
            self.groupbox_streaming.setVisible(True)
        self.settings.setValue('components',components)

            
    def getFileDirInput(self):
        directory = str(QtWidgets.QFileDialog.getExistingDirectory())
        self.lineEdit_fileCache_path.setText('{}'.format(directory))
        # Update the settings 
        filePath = self.settings.value('file_cache')
        filePath['path'] = '{}'.format(directory)
        self.settings.setValue('file_cache',filePath)
        
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

    def populateOptions(self):
        if self.settings.value('components')[1] == 'file_cache':
            self.dropDown_pipeline.setCurrentIndex(pipelineChoices['fileCache'])
        else:
            self.dropDown_pipeline.setCurrentIndex(pipelineChoices['streaming'])
        
        self.dropDown_libfuse_permissions.setCurrentIndex(libfusePermissions[self.settings.value('libfuse')['default-permission']])
        
        if self.settings.value('allow-other') == True:
            self.checkbox_multiUser.setCheckState(Qt.Checked)
        else:
            self.checkbox_multiUser.setCheckState(Qt.Unchecked)
        
        if self.settings.value('nonempty') == True:
            self.checkbox_nonEmptyDir.setCheckState(Qt.Checked)
        else:
            self.checkbox_nonEmptyDir.setCheckState(Qt.Unchecked)            

        if self.settings.value('foreground') == True:
            self.checkbox_daemonForeground.setCheckState(Qt.Checked)
        else:
            self.checkbox_daemonForeground.setCheckState(Qt.Unchecked)  
        
        if self.settings.value('read-only') == True:
            self.checkbox_readOnly.setCheckState(Qt.Checked)
        else:
            self.checkbox_readOnly.setCheckState(Qt.Unchecked)    

        if self.settings.value('stream')['file-caching'] == True:
            self.checkbox_streaming_fileCachingLevel.setCheckState(Qt.Checked)
        else:
            self.checkbox_streaming_fileCachingLevel.setCheckState(Qt.Unchecked)
            
        if self.settings.value('libfuse')['ignore-open-flags'] == True:
            self.checkbox_streaming_fileCachingLevel.setCheckState(Qt.Checked)
        else:
            self.checkbox_streaming_fileCachingLevel.setCheckState(Qt.Unchecked)
            
        self.spinBox_libfuse_attExp.setValue(self.settings.value('libfuse')['attribute-expiration-sec'])
        self.spinBox_libfuse_entExp.setValue(self.settings.value('libfuse')['entry-expiration-sec'])
        self.spinBox_libfuse_negEntryExp.setValue(self.settings.value('libfuse')['negative-entry-expiration-sec'])
        self.spinBox_streaming_blockSize.setValue(self.settings.value('stream')['block-size-mb'])
        self.spinBox_streaming_buffSize.setValue(self.settings.value('stream')['buffer-size-mb'])
        self.spinBox_streaming_maxBuff.setValue(self.settings.value('stream')['max-buffers'])
        
        self.lineEdit_bucketName.setText(self.settings.value('s3storage')['bucket-name'])
        self.lineEdit_endpoint.setText(self.settings.value('s3storage')['endpoint'])
        self.lineEdit_secretKey.setText(self.settings.value('s3storage')['secret-key'])
        self.lineEdit_accessKey.setText(self.settings.value('s3storage')['key-id'])
        self.lineEdit_fileCache_path.setText(self.settings.value('file_cache')['path'])

    def initWindowSizePos(self):
        self.myWindow = QSettings("LyveFUSE", "lycWindow")
        try:
            self.resize(self.myWindow.value("lyc window size"))
            self.move(self.myWindow.value("lyc window position"))
        except:
            pass
    
    def exitWindowCleanup(self):
    # Save this specific window's size and position
        self.myWindow.setValue("lyc window size", self.size())
        self.myWindow.setValue("lyc window position", self.pos())
