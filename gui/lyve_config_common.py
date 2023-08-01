from PySide6.QtCore import Qt, QSettings
from PySide6 import QtWidgets, QtGui

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

        # Allow alphanumeric characters plus [\,/,.,-]
        self.lineEdit_bucketName.setValidator(QtGui.QRegularExpressionValidator("^[a-zA-Z0-9-.\\\/]*$",self))
        # Allow alphanumeric characters plus [/,\,:,.,-,:]
        self.lineEdit_endpoint.setValidator(QtGui.QRegularExpressionValidator("^[a-zA-Z0-9-.\:\\\/]*$",self))
        # Allow alphanumeric characters plus [-,_]
        self.lineEdit_region.setValidator(QtGui.QRegularExpressionValidator("^[a-zA-Z0-9-_]*$",self))
        # Allow alphanumeric characters plus [\,/,-,_]
        self.lineEdit_fileCache_path.setValidator(QtGui.QRegularExpressionValidator("^[a-zA-Z0-9-_\\\/]*$",self))
        
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
        
    def hideModeBoxes(self):
        self.groupbox_fileCache.setVisible(False)
        self.groupbox_streaming.setVisible(False)      

        # Update S3Storage re-writes everything in the S3Storage dictionary for the same reason update libfuse does.
    def updateS3Storage(self):
        s3Storage = self.settings.value('s3storage')
        s3Storage['bucket-name'] = self.lineEdit_bucketName.text()
        s3Storage['key-id'] = self.lineEdit_accessKey.text()
        s3Storage['secret-key'] = self.lineEdit_secretKey.text()
        s3Storage['endpoint'] = self.lineEdit_endpoint.text()
        s3Storage['region'] = self.lineEdit_region.text()
        self.settings.setValue('s3storage',s3Storage) 

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
        
        self.setCheckboxFromSetting(self.checkBox_multiUser, self.settings.value('allow-other'))
        self.setCheckboxFromSetting(self.checkBox_nonEmptyDir,self.settings.value('nonempty'))
        self.setCheckboxFromSetting(self.checkBox_daemonForeground,self.settings.value('foreground'))
        self.setCheckboxFromSetting(self.checkBox_readOnly,self.settings.value('read-only'))
        self.setCheckboxFromSetting(self.checkBox_streaming_fileCachingLevel,stream['file-caching'])
        self.setCheckboxFromSetting(self.checkBox_libfuse_ignoreAppend,libfuse['ignore-open-flags'])

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
        self.lineEdit_region.setText(s3storage['region'])
        self.lineEdit_fileCache_path.setText(fileCache['path'])
        
    def resetDefaults(self):
        # Reset these defaults
        checkChoice = self.popupDoubleCheckReset()
        if checkChoice == QtWidgets.QMessageBox.Yes:
            self.setLyveSettings()
            self.setComponentSettings()
            self.populateOptions()
    
    def updateSettingsFromUIChoices(self):
        self.updateFileCachePath()
        self.updateLibfuse()
        self.updateReadOnly()
        self.updateDaemonForeground()
        self.updateMultiUser()
        self.updateNonEmtpyDir()
        self.updateStream()
        self.updateS3Storage()