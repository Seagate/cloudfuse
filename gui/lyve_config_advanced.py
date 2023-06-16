from PySide6.QtCore import Qt, QSettings
# import the custom class made from QtDesigner
from ui_lyve_config_advanced import Ui_Form
from common_qt_functions import widgetCustomFunctions

file_cache_eviction_choices = ['lru','lfu']

class lyveAdvancedSettingsWidget(widgetCustomFunctions, Ui_Form):
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
        self.button_resetDefaultSettings.clicked.connect(self.populateOptions)

    def populateOptions(self):
        fileCache = self.settings.value('file_cache')
        libfuse = self.settings.value('libfuse')
        
        # The index of file_cache_eviction is matched with the default 
        #   index values in the ui code, so translate the value from settings to index number
        policyIndex = file_cache_eviction_choices.index(fileCache['policy'])
        self.dropDown_fileCache_evictionPolicy.setCurrentIndex(policyIndex)

        self.setCheckboxFromSetting(self.checkbox_libfuse_disableWriteback, libfuse['disable-writeback-cache'])
        self.setCheckboxFromSetting(self.checkbox_fileCache_allowNonEmptyTmp,fileCache['allow-non-empty-temp'])
        self.setCheckboxFromSetting(self.checkbox_fileCache_policyLogs,fileCache['policy-trace'])
        self.setCheckboxFromSetting(self.checkbox_fileCache_createEmptyFile,fileCache['create-empty-file'])
        self.setCheckboxFromSetting(self.checkbox_fileCache_cleanupStart,fileCache['cleanup-on-start'])
        self.setCheckboxFromSetting(self.checkbox_fileCache_offloadIO,fileCache['offload-io'])
        
        self.spinBox_fileCache_evictionTimeout.setValue(fileCache['timeout-sec'])
        self.spinBox_fileCache_maxEviction.setValue(fileCache['max-eviction'])
        self.spinBox_fileCache_maxCacheSize.setValue(fileCache['max-size-mb'])
        self.spinBox_fileCache_evictMaxThresh.setValue(fileCache['high-threshold'])
        self.spinBox_fileCache_evictMinThresh.setValue(fileCache['low-threshold'])

    def updateSettingsFromUIChoices(self):
        self.updateOptionalFileCache()
        self.updateOptionalLibfuse()