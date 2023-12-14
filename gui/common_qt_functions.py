# System imports
import yaml
import os
from sys import platform

# Import QT libraries
from PySide6 import QtWidgets
from PySide6.QtWidgets import QWidget
from PySide6.QtCore import Qt, QSettings

file_cache_eviction_choices = ['lru','lfu']
libfusePermissions = [0o777,0o666,0o644,0o444]

class defaultSettingsManager():
    def __init__(self):
        super().__init__()
        self.settings = QSettings(QSettings.Format.IniFormat,QSettings.Scope.UserScope,"CloudFUSE", "settings")
        self.setAllDefaultSettings()
        
        
    def setAllDefaultSettings(self):
        self.setS3Settings()
        self.setAzureSettings()
        self.setComponentSettings()
        
    def setS3Settings(self):
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
            'virtual-directory': False,
            'disable-compression': False
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
            'ignore-open-flags' : True,
            'max-fuse-threads': 128,
            'network-share': False
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
            'offload-io': False,
            'sync-to-flush': True,
            'refresh-sec': 60
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
            'level' : 'log_err',        
            'file-path' : '$HOME/.cloudfuse/cloudfuse.log',         
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
            if self.writeConfigFile():
                event.accept()
            else:
                event.ignore()
        
    def constructDictForConfig(self):
        optionKeys = self.settings.allKeys()
        configDict = {}
        for key in optionKeys:
            configDict[key] = self.settings.value(key)
        return configDict
    
    def updateSettingsFromUIChoices(self):
        # Each individual widget will need to override this function
        pass

    def getCurrentDir(self):
        defaultFuseDir = 'Cloudfuse'
        if platform == "win32":
            userDir = os.getenv('APPDATA')
            currentDir = os.path.join(userDir, defaultFuseDir)
        else:
            currentDir = os.getcwd()
        return currentDir

    def writeConfigFile(self):
        self.updateSettingsFromUIChoices()
        dictForConfigs = self.constructDictForConfig()
        currentDir = self.getCurrentDir()
        try:
            with open(currentDir+'/config.yaml','w') as file:
                yaml.safe_dump(dictForConfigs,file)
                return True
        except:
            msg = QtWidgets.QMessageBox()
            msg.setWindowTitle("Write Failed")
            msg.setInformativeText("Writing the config file failed. Check file permissions and try again.")
            msg.exec()
            return False
            
    def getConfigs(self,useDefault=False):
        currentDir = self.getCurrentDir()
        if useDefault:
            try:
                with open(currentDir+'/default_config.yaml','r') as file:
                    configs = yaml.safe_load(file)
                    if configs is None:
                        # The default file is empty, use programmed defaults
                        defaultSettingsManager.setAllDefaultSettings(self)
                        configs = self.constructDictForConfig()
            except:
                # There is no default config file, use programmed defaults
                defaultSettingsManager.setAllDefaultSettings(self)
                configs = self.constructDictForConfig()
        else:
            try:
                with open(currentDir+'/config.yaml', 'r') as file:
                    configs = yaml.safe_load(file)
                    if configs is None:
                       # The configs file exists, but is empty, use default settings
                       configs = self.getConfigs(True) 
            except:
                # Could not open or config file does not exist, use default settings
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

    def updateMultiUser(self):
        self.settings.setValue('allow-other',self.checkBox_multiUser.isChecked())
        
    def updateNonEmtpyDir(self):
        self.settings.setValue('nonempty',self.checkBox_nonEmptyDir.isChecked())        
    
    def updateDaemonForeground(self):
        self.settings.setValue('foreground',self.checkBox_daemonForeground.isChecked())
    
    def updateReadOnly(self):
        self.settings.setValue('read-only',self.checkBox_readOnly.isChecked())
    
    # Update Libfuse re-writes everything in the Libfuse because of how setting.setValue works - 
    #   it will not append, so the code makes a copy of the dictionary and updates the sub-keys. 
    #   When the user updates the sub-option through the GUI, it will trigger Libfuse to update;
    #   it's written this way to save on lines of code.
    def updateLibfuse(self):
        libfuse = self.settings.value('libfuse')
        libfuse['default-permission'] = libfusePermissions[self.dropDown_libfuse_permissions.currentIndex()]
        libfuse['ignore-open-flags'] = self.checkBox_libfuse_ignoreAppend.isChecked()
        libfuse['attribute-expiration-sec'] = self.spinBox_libfuse_attExp.value()
        libfuse['entry-expiration-sec'] = self.spinBox_libfuse_entExp.value()
        libfuse['negative-entry-expiration-sec'] = self.spinBox_libfuse_negEntryExp.value()
        self.settings.setValue('libfuse',libfuse)

    def updateOptionalLibfuse(self):
        libfuse = self.settings.value('libfuse')
        libfuse['disable-writeback-cache'] = self.checkBox_libfuse_disableWriteback.isChecked()
        libfuse['network-share'] = self.checkBox_libfuse_networkshare.isChecked()
        libfuse['max-fuse-threads'] = self.spinBox_libfuse_maxFuseThreads.value()
        self.settings.setValue('libfuse',libfuse)

    # Update stream re-writes everything in the stream dictionary for the same reason update libfuse does.
    def updateStream(self):
        stream = self.settings.value('stream')
        stream['file-caching'] = self.checkBox_streaming_fileCachingLevel.isChecked()
        stream['block-size-mb'] = self.spinBox_streaming_blockSize.value()
        stream['buffer-size-mb'] = self.spinBox_streaming_buffSize.value()
        stream['max-buffers'] = self.spinBox_streaming_maxBuff.value()
        self.settings.setValue('stream',stream)
     
    def updateFileCachePath(self):
        filePath = self.settings.value('file_cache')
        filePath['path'] = self.lineEdit_fileCache_path.text()
        self.settings.setValue('file_cache',filePath)
        
    def updateOptionalFileCache(self):
        fileCache = self.settings.value('file_cache')
        fileCache['allow-non-empty-temp'] = self.checkBox_fileCache_allowNonEmptyTmp.isChecked()
        fileCache['policy-trace'] = self.checkBox_fileCache_policyLogs.isChecked()
        fileCache['create-empty-file'] = self.checkBox_fileCache_createEmptyFile.isChecked()
        fileCache['cleanup-on-start'] = self.checkBox_fileCache_cleanupStart.isChecked()
        fileCache['offload-io'] = self.checkBox_fileCache_offloadIO.isChecked()
        fileCache['sync-to-flush'] = self.checkBox_fileCache_syncToFlush.isChecked()
        
        fileCache['timeout-sec'] = self.spinBox_fileCache_evictionTimeout.value()
        fileCache['max-eviction'] = self.spinBox_fileCache_maxEviction.value()
        fileCache['max-size-mb'] = self.spinBox_fileCache_maxCacheSize.value()
        fileCache['high-threshold'] = self.spinBox_fileCache_evictMaxThresh.value()
        fileCache['low-threshold'] = self.spinBox_fileCache_evictMinThresh.value()
        fileCache['refresh-sec'] = self.spinBox_fileCache_refreshSec.value()
        
        fileCache['policy'] = file_cache_eviction_choices[self.dropDown_fileCache_evictionPolicy.currentIndex()]
        self.settings.setValue('file_cache',fileCache)