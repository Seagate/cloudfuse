from PySide6 import QtWidgets
from PySide6.QtWidgets import QWidget
from PySide6.QtCore import Qt, QSettings
import yaml
import os

class defaultSettingsManager():
    def __init__(self):
        super().__init__()
        self.settings = QSettings("LyveFUSE", "settings")
        self.setAllDefaultSettings()
        
        
    def setAllDefaultSettings(self):
        self.setLyveSettings()
        self.setAzureSettings()
        self.setComponentSettings()
        
    def setLyveSettings(self):
        # REFER TO ~/setup/baseConfig.yaml for explanations of what these settings are
        self.settings.setValue('s3storage',{
            'bucket-name': '',
            'key-id': '',
            'secret-key': '',
            'region': '',
            'endpoint': '',
            'subdirectory': ''
        })
    
    def setAzureSettings(self):
        # REFER TO ~/setup/baseConfig.yaml for explanations of what these settings are
        self.settings.setValue('azstorage',{
            'type': 'block',
            'account-name': '',
            'container': '',
            'endpoint': '',
            'mode': 'key',
            'account-key': '',
            'sas': '',
            'appid': '',
            'resid': '',
            'objid': '',
            'tenantid': '',
            'clientid': '',
            'clientsecret': '',
            'use-http': False,
            'aadendpoint': '',
            'subdirectory': '',
            'block-size-mb': 16,
            'max-concurrency': 32,
            'tier': 'none',
            'block-list-on-mount-sec': 0,
            'max-retries': 5,
            'max-retry-timeout-sec': 900,
            'retry-backoff-sec': 4,
            'max-retry-delay-sec': 60,
            'http-proxy': '',
            'https-proxy': '',
            'sdk-trace': False,
            'fail-unsupported-op': False,
            'auth-resource': '',
            'update-md5': False,
            'validate-md5': False,
            'virtual-directory': False
            # TODO: disable-compression flag is missing
        })
    
    def setComponentSettings(self):
        # REFER TO ~/setup/baseConfig.yaml for explanations of what these settings are
        self.settings.setValue('foreground',False)
        self.settings.setValue('allow-other',True)
        self.settings.setValue('read-only',False)
        self.settings.setValue('nonempty',False)
        self.settings.setValue('dynamic-profile',False)
        self.settings.setValue('profiler-port',6060)
        self.settings.setValue('profiler-ip','localhost')
        # This is the built pipeline name components 
        self.settings.setValue('components',['libfuse','file_cache','attr_cache','azstorage'])
        self.settings.setValue('libfuse',{
            'default-permission' : 0o777,
            'attribute-expiration-sec': 120,
            'entry-expiration-sec' : 120,   
            'negative-entry-expiration-sec' : 120,
            'fuse-trace' : False,
            'extension' : '', 
            'disable-writeback-cache' : False,
            'ignore-open-flags' : True
            # TODO: max-fuse-threads and network-share are missing
        })
        self.settings.setValue('stream',{
            'block-size-mb': 0,
            'max-buffers': 0,
            'buffer-size-mb': 0,
            'file-caching': False # false = handle level caching ON
        })
        self.settings.setValue('file_cache',{
            'path': '',
            'policy': 'lru',
            'timeout-sec' : 120,
            'max-eviction': 5000,
            'max-size-mb': 0,
            'high-threshold': 80,
            'low-threshold': 60,
            'create-empty-file': False,
            'allow-non-empty-temp': False,
            'cleanup-on-start': False,
            'policy-trace': False,
            'offload-io': False
            # TODO: sync-to-flush is missing
        })
        self.settings.setValue('attr_cache',{
            'timeout-sec': 120,
            'no-cache-on-list': False,
            'no-symlinks': False
        })
        self.settings.setValue('loopbackfs',{
            'path': ''
        })
        
        self.settings.setValue('mountall',{
            'container-allowlist': [],
            'container-denylist': []
        })
        self.settings.setValue('health_monitor',{
            'enable-monitoring': False,
            'stats-poll-interval-sec': 10,
            'process-monitor-interval-sec': 30,
            'output-path':'',
            'monitor-disable-list': [
                'blobfuse_stats',
                'file_cache_monitor',
                'cpu_profiler',
                'memory_profiler',
                'network_profiler'
                ]
        })
        self.settings.setValue('logging',{
            'type' : 'syslog',
            'level' : 'log_warning',        
            'file-path' : '$HOME/.lyvecloudfuse/lyvecloudfuse.log',         
            'max-file-size-mb' : 512,                                       
            'file-count' : 10 ,                                             
            'track-time' : False                                            
            })
    
class widgetCustomFunctions(QWidget):
    def __init__(self):
        super().__init__()
        
    def exitWindow(self):
        self.close()
        
    def exitWindowCleanup(self):
    # Save this specific window's size and position
        self.myWindow.setValue("window size", self.size())
        self.myWindow.setValue("window position", self.pos())
            
    def popupDoubleCheckReset(self):
        checkMsg = QtWidgets.QMessageBox()
        checkMsg.setWindowTitle("Are you sure?")
        checkMsg.setInformativeText("ResetDefault settings will reset all settings for this target.")
        checkMsg.setStandardButtons(QtWidgets.QMessageBox.Cancel | QtWidgets.QMessageBox.Yes)
        checkMsg.setDefaultButton(QtWidgets.QMessageBox.Cancel)
        choice = checkMsg.exec()
        return choice
            
    # Overrides the closeEvent function from parent class to enable this custom behavior
    # TODO: Nice to have - keep track of changes to user makes and only trigger the 'are you sure?' message 
    #   when changes have been made
    def closeEvent(self, event):
        msg = QtWidgets.QMessageBox()
        msg.setWindowTitle("Are you sure?")
        msg.setInformativeText("Do you want to save you changes?")
        msg.setText("The settings have been modified.")
        msg.setStandardButtons(QtWidgets.QMessageBox.Discard | QtWidgets.QMessageBox.Cancel | QtWidgets.QMessageBox.Save)
        msg.setDefaultButton(QtWidgets.QMessageBox.Cancel)
        ret = msg.exec()
        
        if ret == QtWidgets.QMessageBox.Discard:
            self.exitWindowCleanup()
            event.accept()
        elif ret == QtWidgets.QMessageBox.Cancel:
            event.ignore()
        elif ret == QtWidgets.QMessageBox.Save:
            # Insert all settings to yaml file
            self.exitWindowCleanup()
            self.writeConfigFile()
            event.accept()
        
    def constructDictForConfig(self):
        optionKeys = self.settings.allKeys()
        configDict = {}
        for key in optionKeys:
            configDict[key] = self.settings.value(key)
        return configDict
    
    def updateSettingsFromUIChoices(self):
        # Each individual widget will need to override this function
        pass

    def writeConfigFile(self):
        self.updateSettingsFromUIChoices()
        dictForConfigs = self.constructDictForConfig()
        currentDir = os.getcwd()
        with open(currentDir+'/config.yaml','w') as file:
            yaml.safe_dump(dictForConfigs,file)
            
    def getConfigs(self,useDefault=False):
        currentDir = os.getcwd()
        if useDefault:
            try:
                with open(currentDir+'/default_config.yaml','r') as file:
                    configs = yaml.safe_load(file)
            except:
                # There is no default config file, use programmed defaults
                defaultSettingsManager.setAllDefaultSettings(self)
                configs = self.constructDictForConfig()
        else:
            try:
                with open(currentDir+'/config.yaml', 'r') as file:
                    configs = yaml.safe_load(file)
            except:
                configs = self.getConfigs(True)
        return configs
    
    
    def initWindowSizePos(self):
        try:
            self.resize(self.myWindow.value("window size"))
            self.move(self.myWindow.value("window position"))
        except:
            pass
        
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

    # Check for a true/false setting and set the checkbox state as appropriate. 
    #   Note, Checked/UnChecked are NOT True/False data types, hence the need to check what the values are.
    #   The default values for True/False settings are False, which is why Unchecked is the default state if the value doesn't equate to True.
    #   Explicitly check for True for clarity
    def setCheckboxFromSetting(self, checkbox, settingName):
        if settingName == True:
            checkbox.setCheckState(Qt.Checked)
        else:
            checkbox.setCheckState(Qt.Unchecked)
