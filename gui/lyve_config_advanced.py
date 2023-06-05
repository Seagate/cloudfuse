from PySide6.QtCore import Qt, QSettings
# import the custom class made from QtDesigner
from ui_lyve_config_advanced import Ui_Form
from common_qt_functions import commonConfigFunctions

file_cache_eviction_choices = ['lru','lfu']

class lyveAdvancedSettingsWidget(commonConfigFunctions, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.myWindow = QSettings("LyveFUSE", "lycAdvancedWindow")
        # Get the config settings from the QSettings repo - do not inherit from defaultManager, it resets the settings to default
        self.settings = QSettings("LyveFUSE", "settings")
        
        self.initWindowSizePos()
        self.setWindowTitle("Advanced LyveCloud Config Settings")
        self.populateOptions()
        
        # Set up the signals
        self.button_okay.clicked.connect(self.exitWindow)
        self.button_resetDefaultSettings.clicked.connect(self.resetDefaults)

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
        
        fileCache['timeout-sec'] = self.spinBox_fileCache_evictionTimeout.value()
        fileCache['max-eviction'] = self.spinBox_fileCache_maxEviction.value()
        fileCache['max-size-mb'] = self.spinBox_fileCache_maxCacheSize.value()
        fileCache['high-threshold'] = self.spinBox_fileCache_evictMaxThresh.value()
        fileCache['low-threshold'] = self.spinBox_fileCache_evictMinThresh.value()
        
        fileCache['policy'] = file_cache_eviction_choices[self.dropDown_fileCache_evictionPolicy.currentIndex()]
        self.settings.setValue('file_cache',fileCache)
        
        
    def populateOptions(self):
        policyIndex = file_cache_eviction_choices.index(self.settings.value('file_cache')['policy'])
        self.dropDown_fileCache_evictionPolicy.setCurrentIndex(policyIndex)
        
        if self.settings.value('libfuse')['disable-writeback-cache'] == True:
            self.checkbox_libfuse_disableWriteback.setCheckState(Qt.Checked)
        else:
            self.checkbox_libfuse_disableWriteback.setCheckState(Qt.Unchecked)    
        
        if self.settings.value('file_cache')['allow-non-empty-temp'] == True:
            self.checkbox_fileCache_allowNonEmptyTmp.setCheckState(Qt.Checked)
        else:
            self.checkbox_fileCache_allowNonEmptyTmp.setCheckState(Qt.Unchecked)
        
        if self.settings.value('file_cache')['policy-trace'] == True:
            self.checkbox_fileCache_policyLogs.setCheckState(Qt.Checked)
        else:
            self.checkbox_fileCache_policyLogs.setCheckState(Qt.Unchecked)
        
        if self.settings.value('file_cache')['create-empty-file'] == True:
            self.checkbox_fileCache_createEmptyFile.setCheckState(Qt.Checked)
        else:
            self.checkbox_fileCache_createEmptyFile.setCheckState(Qt.Unchecked)    
            
        if self.settings.value('file_cache')['cleanup-on-start'] == True:
            self.checkbox_fileCache_cleanupStart.setCheckState(Qt.Checked)
        else:
            self.checkbox_fileCache_cleanupStart.setCheckState(Qt.Unchecked)
        
        if self.settings.value('file_cache')['offload-io'] == True:
            self.checkbox_fileCache_offloadIO.setCheckState(Qt.Checked)
        else:
            self.checkbox_fileCache_offloadIO.setCheckState(Qt.Unchecked)

        self.spinBox_fileCache_evictionTimeout.setValue(self.settings.value('file_cache')['timeout-sec'])
        self.spinBox_fileCache_maxEviction.setValue(self.settings.value('file_cache')['max-eviction'])
        self.spinBox_fileCache_maxCacheSize.setValue(self.settings.value('file_cache')['max-size-mb'])
        self.spinBox_fileCache_evictMaxThresh.setValue(self.settings.value('file_cache')['high-threshold'])
        self.spinBox_fileCache_evictMinThresh.setValue(self.settings.value('file_cache')['low-threshold'])

    def resetDefaults(self):
        # Fill in the default values for advanced
        pass

    def updateSettingsFromUIChoices(self):
        self.updateFileCache()
        self.updateLibfuse()