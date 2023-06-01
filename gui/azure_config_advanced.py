from PySide6.QtCore import Qt, QSettings
from PySide6.QtWidgets import QWidget
from PySide6 import QtWidgets
# import the custom class made from QtDesigner
from ui_azure_config_advanced import Ui_Form
from common_qt_functions import commonConfigFunctions

file_cache_eviction_choices = ['lru','lfu']
az_blob_tier = ['hot','cool','archive','none']

class azureAdvancedSettingsWidget(commonConfigFunctions, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle("Advanced Azure Config Settings")
        self.settings = QSettings("LyveFUSE", "settings")
        self.myWindow = QSettings("LyveFUSE", "AzAdvancedWindow")
        self.initWindowSizePos()
        self.populateOptions()
        
        # Set up the signals
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_resetDefaultSettings.clicked.connect(self.resetDefaults)
        

    def populateOptions(self):
        fileCache = self.settings.value('file_cache')
        azStorage = self.settings.value('azstorage')

        if self.settings.value('libfuse')['disable-writeback-cache'] == True:
            self.checkbox_libfuse_disableWriteback.setCheckState(Qt.Checked)
        else:
            self.checkbox_libfuse_disableWriteback.setCheckState(Qt.Unchecked) 
       
        if fileCache['allow-non-empty-temp'] == True:
            self.checkbox_fileCache_allowNonEmptyTmp.setCheckState(Qt.Checked)
        else:
            self.checkbox_fileCache_allowNonEmptyTmp.setCheckState(Qt.Unchecked)
            
        if fileCache['policy-trace'] == True:
            self.checkbox_fileCache_policyLogs.setCheckState(Qt.Checked)
        else:
            self.checkbox_fileCache_policyLogs.setCheckState(Qt.Unchecked)
            
        if fileCache['create-empty-file'] == True:
            self.checkbox_fileCache_createEmptyFile.setCheckState(Qt.Checked)
        else:
            self.checkbox_fileCache_createEmptyFile.setCheckState(Qt.Unchecked)

        if fileCache['cleanup-on-start'] == True:
            self.checkbox_fileCache_cleanupStart.setCheckState(Qt.Checked)
        else:
            self.checkbox_fileCache_cleanupStart.setCheckState(Qt.Unchecked)
            
        if fileCache['offload-io'] == True:
            self.checkbox_fileCache_offloadIO.setCheckState(Qt.Checked)
        else:
            self.checkbox_fileCache_offloadIO.setCheckState(Qt.Unchecked)

        if azStorage['use-http'] == True:
            self.checkbox_azure_useHttp.setCheckState(Qt.Checked)
        else:
            self.checkbox_azure_useHttp.setCheckState(Qt.Unchecked)
    
        if azStorage['validate-md5'] == True:
            self.checkbox_azure_validateMd5.setCheckState(Qt.Checked)
        else:
            self.checkbox_azure_validateMd5.setCheckState(Qt.Unchecked)
            
        if azStorage['update-md5'] == True:
            self.checkbox_azure_updateMd5.setCheckState(Qt.Checked)
        else:
            self.checkbox_azure_updateMd5.setCheckState(Qt.Unchecked)
            
        if azStorage['fail-unsupported-op'] == True:
            self.checkbox_azure_failUnsupportedOps.setCheckState(Qt.Checked)
        else:
            self.checkbox_azure_failUnsupportedOps.setCheckState(Qt.Unchecked)

        if azStorage['sdk-trace'] == True:
            self.checkbox_azure_sdkTrace.setCheckState(Qt.Checked)
        else:
            self.checkbox_azure_sdkTrace.setCheckState(Qt.Unchecked)
            
        if azStorage['virtual-directory'] == True:
            self.checkbox_azure_virtualDirectory.setCheckState(Qt.Checked)
        else:
            self.checkbox_azure_virtualDirectory.setCheckState(Qt.Unchecked)  
        
        self.spinBox_fileCache_timeout.setValue(fileCache['timeout-sec'])
        self.spinBox_fileCache_maxEviction.setValue(fileCache['max-eviction'])
        self.spinBox_fileCache_maxCacheSize.setValue(fileCache['max-size-mb'])
        self.spinBox_fileCache_evictMaxThresh.setValue(fileCache['high-threshold'])
        self.spinBox_fileCache_evictMinThresh.setValue(fileCache['low-threshold'])
        self.spinBox_azure_blockSize.setValue(azStorage['block-size-mb'])
        self.spinBox_azure_maxConcurrency.setValue(azStorage['max-concurrency'])
        self.spinBox_azure_blockOnMount.setValue(azStorage['block-list-on-mount-sec'])
        self.spinBox_azure_maxRetries.setValue(azStorage['max-retries'])
        self.spinBox_azure_maxRetryTimeout.setValue(azStorage['max-retry-timeout-sec'])
        self.spinBox_azure_retryBackoff.setValue(azStorage['retry-backoff-sec'])
        self.spinBox_azure_maxRetryDelay.setValue(azStorage['max-retry-delay-sec'])

        self.lineEdit_azure_aadEndpoint.setText(azStorage['aadendpoint'])
        self.lineEdit_azure_subDirectory.setText(azStorage['subdirectory'])
        self.lineEdit_azure_httpProxy.setText(azStorage['http-proxy'])
        self.lineEdit_azure_httpsProxy.setText(azStorage['https-proxy'])
        self.lineEdit_azure_authResource.setText(azStorage['auth-resource'])
        self.dropDown_azure_blobTier.setCurrentIndex(az_blob_tier.index(azStorage['tier']))
        self.dropDown_fileCache_evictionPolicy.setCurrentIndex(file_cache_eviction_choices.index(fileCache['policy']))


    def updateLibfuse(self):
        libfuse = self.settings.value('libfuse')
        libfuse['disable-writeback-cache'] = self.checkbox_libfuse_disableWriteback.isChecked()
        self.settings.setValue('libfuse',libfuse)
        
    def updateFileCache(self):
        fileCache = self.settings.value('file_cache')
        fileCache['allow-non-empty-temp'] = self.checkbox_fileCache_allowNonEmptyTmp.isChecked()
        fileCache['policy-trace'] = self.checkbox_fileCache_policyLogs.isChecked()
        fileCache['create-empty-file'] = self.checkbox_fileCache_createEmptyFile.isChecked()
        fileCache['cleanup-on-start'] = self.checkbox_fileCache_cleanupStart.isChecked()
        fileCache['offload-io'] = self.checkbox_fileCache_offloadIO.isChecked()
        fileCache['timeout-sec'] = self.spinBox_fileCache_timeout.value()
        fileCache['max-eviction'] = self.spinBox_fileCache_maxEviction.value()
        fileCache['max-size-mb'] = self.spinBox_fileCache_maxCacheSize.value()
        fileCache['high-threshold'] = self.spinBox_fileCache_evictMaxThresh.value()
        fileCache['low-threshold'] = self.spinBox_fileCache_evictMinThresh.value()
        fileCache['policy'] = file_cache_eviction_choices[self.dropDown_fileCache_evictionPolicy.currentIndex()]
        self.settings.setValue('file_cache',fileCache)

    def updateAzStorage(self):
        azStorage = self.settings.value('azstorage')
        azStorage['block-size-mb'] = self.spinBox_azure_blockSize.value()
        azStorage['max-concurrency'] = self.spinBox_azure_maxConcurrency.value()
        azStorage['block-list-on-mount-sec'] = self.spinBox_azure_blockOnMount.value()
        azStorage['max-retries'] = self.spinBox_azure_maxRetries.value()
        azStorage['max-retry-timeout-sec'] = self.spinBox_azure_maxRetryTimeout.value()
        azStorage['retry-backoff-sec'] = self.spinBox_azure_retryBackoff.value()
        azStorage['max-retry-delay-sec'] = self.spinBox_azure_maxRetryDelay.value()
        azStorage['use-http'] = self.checkbox_azure_useHttp.isChecked()
        azStorage['validate-md5'] = self.checkbox_azure_validateMd5.isChecked()
        azStorage['update-md5'] = self.checkbox_azure_updateMd5.isChecked()
        azStorage['fail-unsupported-op'] = self.checkbox_azure_failUnsupportedOps.isChecked()
        azStorage['sdk-trace'] = self.checkbox_azure_sdkTrace.isChecked()
        azStorage['virtual-directory'] = self.checkbox_azure_virtualDirectory.isChecked()
        azStorage['aadendpoint'] = self.lineEdit_azure_aadEndpoint.text()
        azStorage['subdirectory'] = self.lineEdit_azure_subDirectory.text()
        azStorage['http-proxy'] = self.lineEdit_azure_httpProxy.text()
        azStorage['https-proxy'] = self.lineEdit_azure_httpsProxy.text()
        azStorage['auth-resource'] = self.lineEdit_azure_authResource.text()
        azStorage['tier'] = az_blob_tier[self.dropDown_azure_blobTier.currentIndex()]
        self.settings.setValue('azstorage',azStorage)
    
    def resetDefaults(self):
        # Fill in the default values for advanced
        pass

    def writeConfigFile(self):
        self.updateAzStorage()
        self.updateFileCache()
        self.updateLibfuse()
