from PySide6.QtCore import Qt, QSettings
from PySide6 import QtWidgets

# import the custom class made from QtDesigner
from ui_lyve_config_common import Ui_Form
from lyve_config_advanced import lyveAdvancedSettingsWidget
from common_qt_functions import defaultSettingsManager,widgetCustomFunctions

pipelineChoices = ['file_cache','stream']
libfusePermissions = [0o777,0o666,0o644,0o444]

class lyveSettingsWidget(defaultSettingsManager,widgetCustomFunctions,Ui_Form): 
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
        self.button_browse.clicked.connect(self.getFileDirInput)
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_advancedSettings.clicked.connect(self.openAdvanced)
        self.button_resetDefaultSettings.clicked.connect(self.resetDefaults)
       
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
        libfuse['default-permission'] = libfusePermissions[self.dropDown_libfuse_permissions.currentIndex()]
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
        components = self.settings.value('components')
        pipelineIndex = self.dropDown_pipeline.currentIndex()
        components[1] = pipelineChoices[pipelineIndex]
        if pipelineChoices[pipelineIndex] == 'file_cache':
            self.groupbox_fileCache.setVisible(True)
        elif pipelineChoices[pipelineIndex] == 'stream':
            self.groupbox_streaming.setVisible(True)
        self.settings.setValue('components',components)

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

    # This widget will not display all the options in settings, only the ones written in the UI file.
    def populateOptions(self):
        fileCache = self.settings.value('file_cache')
        s3storage = self.settings.value('s3storage')
        libfuse = self.settings.value('libfuse')
        stream = self.settings.value('stream')
                
        # The QCombo (dropdown selection) uses indices to determine the value to show the user. The pipelineChoices and libfusePermissions reflect the 
        #   index choices in human words without having to reference the UI. Get the value in the settings and translate that to the equivalent index in the lists.
        self.dropDown_pipeline.setCurrentIndex(pipelineChoices.index(self.settings.value('components')[1]))
        self.dropDown_libfuse_permissions.setCurrentIndex(libfusePermissions.index(libfuse['default-permission']))
        
        self.setCheckboxFromSetting(self.checkbox_multiUser, self.settings.value('allow-other'))
        self.setCheckboxFromSetting(self.checkbox_nonEmptyDir,self.settings.value('nonempty'))
        self.setCheckboxFromSetting(self.checkbox_daemonForeground,self.settings.value('foreground'))
        self.setCheckboxFromSetting(self.checkbox_readOnly,self.settings.value('read-only'))
        self.setCheckboxFromSetting(self.checkbox_streaming_fileCachingLevel,stream['file-caching'])
        self.setCheckboxFromSetting(self.checkbox_libfuse_ignoreAppend,libfuse['ignore-open-flags'])

        # Spinbox automatically sanitizes intputs for decimal values only, so no need to check for the appropriate data type. 
        self.spinBox_libfuse_attExp.setValue(libfuse['attribute-expiration-sec'])
        self.spinBox_libfuse_entExp.setValue(libfuse['entry-expiration-sec'])
        self.spinBox_libfuse_negEntryExp.setValue(libfuse['negative-entry-expiration-sec'])
        self.spinBox_streaming_blockSize.setValue(stream['block-size-mb'])
        self.spinBox_streaming_buffSize.setValue(stream['buffer-size-mb'])
        self.spinBox_streaming_maxBuff.setValue(stream['max-buffers'])
        # TODO:
        # There is no sanitizing for lineEdit at the moment, the GUI depends on the user being correc.
        self.lineEdit_bucketName.setText(s3storage['bucket-name'])
        self.lineEdit_endpoint.setText(s3storage['endpoint'])
        self.lineEdit_secretKey.setText(s3storage['secret-key'])
        self.lineEdit_accessKey.setText(s3storage['key-id'])
        self.lineEdit_fileCache_path.setText(fileCache['path'])
        
    def resetDefaults(self):
        # Reset these defaults
        checkChoice = self.popupDoubleCheckReset()
        if checkChoice == QtWidgets.QMessageBox.Yes:
            self.setLyveSettings()
            self.setComponentSettings()
            self.populateOptions()
    
    def updateSettingsFromUIChoices(self):
        self.updateFileCache()
        self.updateLibfuse()
        self.updateReadOnly()
        self.updateDaemonForeground()
        self.updateMultiUser()
        self.updateNonEmtpyDir()
        self.updateStream()
        self.updateS3Storage()