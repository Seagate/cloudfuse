from PySide6.QtCore import Qt, QSettings
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets

# import the custom class made from QtDesigner
from ui_lyve_config_common import Ui_Form
from lyve_config_advanced import lyveAdvancedSettingsWidget
from common_qt_functions import defaultSettingsManager,commonConfigFunctions

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

class lyveSettingsWidget(defaultSettingsManager,commonConfigFunctions,Ui_Form): 
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.myWindow = QSettings("LyveFUSE", "lycWindow")
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

       
    # Set up slots for the signals:
    
    def updateMultiUser(self):
        self.settings.setValue('allow-other',self.checkbox_multiUser.isChecked())
        
    def updateNonEmtpyDir(self):
        self.settings.setValue('nonempty',self.checkbox_nonEmptyDir.isChecked())        
    
    def updateDaemonForeground(self):
        self.settings.setValue('foreground',self.checkbox_daemonForeground.isChecked())
    
    def updateReadOnly(self):
        self.settings.setValue('read-only',self.checkbox_readOnly.isChecked())
    
    # Update Libfuse re-writes everything in the Libfuse because of how setting.setValue works - 
    #   it will not append, so the code makes a copy of the dictionary and updates the sub-keys. 
    #   When the user updates the sub-option through the GUI, it will trigger Libfuse to update;
    #   it's written this way to save on lines of code.
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

    # Update stream re-writes everything in the stream dictionary for the same reason update libfuse does.
    def updateStream(self):
        stream = self.settings.value('stream')
        stream['file-caching'] = self.checkbox_streaming_fileCachingLevel.isChecked()
        stream['block-size-mb'] = self.spinBox_streaming_blockSize.value()
        stream['buffer-size-mb'] = self.spinBox_streaming_buffSize.value()
        stream['max-buffers'] = self.spinBox_streaming_maxBuff.value()
        self.settings.setValue('stream',stream)
       
    # Update S3Storage re-writes everything in the S3Storage dictionary for the same reason update libfuse does.
    def updateS3Storage(self):
        s3Storage = self.settings.value('s3storage')
        s3Storage['bucket-name'] = self.lineEdit_bucketName.text()
        s3Storage['key-id'] = self.lineEdit_accessKey.text()
        s3Storage['secret-key'] = self.lineEdit_secretKey.text()
        s3Storage['endpoint'] = self.lineEdit_endpoint.text()
        self.settings.setValue('s3storage',s3Storage)
        
    # To open the advanced widget, make an instance, so self.moresettings was chosen.
    #   self.moresettings does not have anything to do with the QSettings package that is seen throughout this code
    def openAdvanced(self):
        self.moreSettings = lyveAdvancedSettingsWidget()
        self.moreSettings.setWindowModality(Qt.ApplicationModal)
        self.moreSettings.show()

    # ShowModeSettings will switch which groupbox is visiible: stream or file_cache
    #   the function also updates the internal components settings through QSettings
    #   There is one slot for the signal to be pointed at which is why showmodesettings is used.
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

    # defaultSettingsManager has set the settings to all default, now the code needs to pull in
    #   all the changes from the config file the user provides. This may not include all the 
    #   settings defined in defaultSettingManager.
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

    # This widget will not display all the options in settings, only the ones written in the UI file.
    def populateOptions(self):
        if self.settings.value('components')[1] == 'file_cache':
            self.dropDown_pipeline.setCurrentIndex(pipelineChoices['fileCache'])
        else:
            self.dropDown_pipeline.setCurrentIndex(pipelineChoices['streaming'])
        
        self.dropDown_libfuse_permissions.setCurrentIndex(libfusePermissions[self.settings.value('libfuse')['default-permission']])
        
        # Check for a true/false setting and set the checkbox state as appropriate. 
        #   Note, Checked/UnChecked are NOT True/False data types, hence the need to check what the values are.
        #   The default values for True/False settings are False, which is why Unchecked is the default state.
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
        
        # Spinbox automatically sanitizes intputs for decimal values only, so no need to check for the appropriate data type. 
        self.spinBox_libfuse_attExp.setValue(self.settings.value('libfuse')['attribute-expiration-sec'])
        self.spinBox_libfuse_entExp.setValue(self.settings.value('libfuse')['entry-expiration-sec'])
        self.spinBox_libfuse_negEntryExp.setValue(self.settings.value('libfuse')['negative-entry-expiration-sec'])
        self.spinBox_streaming_blockSize.setValue(self.settings.value('stream')['block-size-mb'])
        self.spinBox_streaming_buffSize.setValue(self.settings.value('stream')['buffer-size-mb'])
        self.spinBox_streaming_maxBuff.setValue(self.settings.value('stream')['max-buffers'])
        
        # There is no sanitizing for lineEdit at the moment, the GUI depends on the user being correc.
        self.lineEdit_bucketName.setText(self.settings.value('s3storage')['bucket-name'])
        self.lineEdit_endpoint.setText(self.settings.value('s3storage')['endpoint'])
        self.lineEdit_secretKey.setText(self.settings.value('s3storage')['secret-key'])
        self.lineEdit_accessKey.setText(self.settings.value('s3storage')['key-id'])
        self.lineEdit_fileCache_path.setText(self.settings.value('file_cache')['path'])