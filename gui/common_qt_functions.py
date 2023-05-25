from PySide6 import QtWidgets
from PySide6.QtWidgets import QWidget
from PySide6.QtCore import QSettings
import yaml
import os

class defaultSettingsManager():
        def __init__(self):
            super().__init__()
            self.settings = QSettings("LyveFUSE", "settings")
            self.setDefaultSettings()
            
        def setDefaultSettings(self):
            # REFER TO ~/setup/baseConfig.yaml for explanations of what these settings are
            self.settings.setValue('foreground',False)
            self.settings.setValue('allow-other',True)
            self.settings.setValue('read-only',False)
            self.settings.setValue('nonempty',False)
            self.settings.setValue('dynamic-profile',False)
            self.settings.setValue('profiler-port',6060)
            self.settings.setValue('profiler-ip','localhost')
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
            })
            self.settings.setValue('stream',{
                'block-size-mb': 0,
                'max-buffers': 0,
                'buffer-size-mb': 0,
                'file-caching': False # false - handle level caching ON
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
            })
            self.settings.setValue('attr_cache',{
                'timeout-sec': 120,
                'no-cache-on-list': False,
                'no-symlinks': False
            })
            self.settings.setValue('loopbackfs',{
                'path': ''
            })
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
            })
            self.settings.setValue('s3storage',{
                'bucket-name': '',
                'key-id': '',
                'secret-key': '',
                'region': '',
                'endpoint': '',
                'subdirectory': ''
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
            
class commonConfigFunctions(QWidget):
    def __init__(self):
        super().__init__()
        
    def exitWindow(self):
        self.close()
        
    def exitWindowCleanup(self):
    # Save this specific window's size and position
        self.myWindow.setValue("window size", self.size())
        self.myWindow.setValue("window position", self.pos())
            
    # Override the closeEvent function from parent class to enable custom behavior
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

    def writeConfigFile(self):
        dictForConfigs = self.constructDictForConfig()
        currentDir = os.getcwd()
        with open(currentDir+'/testing_config.yaml','w') as file:
            yaml.safe_dump(dictForConfigs,file)
            
    def getConfigs(self,useDefault=False):
        currentDir = os.getcwd()
        if useDefault:
                with open(currentDir+'/default_config.yaml','r') as file:
                    configs = yaml.safe_load(file)
        else:
            try:
                with open(currentDir+'/config.yaml', 'r') as file:
                    configs = yaml.safe_load(file,)
            except:
                configs = self.getConfigs(True)
        return configs
    
    
    def initWindowSizePos(self):
        try:
            self.resize(self.myWindow.value("window size"))
            self.move(self.myWindow.value("window position"))
        except:
            pass