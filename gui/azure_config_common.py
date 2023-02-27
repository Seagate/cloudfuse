from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget

# import the custom class made from QtDesigner
from ui_azure_config_common import Ui_Form

class azureSettingsWidget(QWidget, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)

        # Hide the pipeline mode groupbox depending on the default select is
        self.showAzureModeSettings()
        self.showModeSettings()

        # Set up signals
        self.pipeline_select.currentIndexChanged.connect(self.showModeSettings)
        self.azModesettings_select.currentIndexChanged.connect(self.showAzureModeSettings)


    # Set up slots

    def showModeSettings(self):

        if self.pipeline_select.currentIndex() == 0:
            self.streaming_groupbox.setVisible(False)
            self.filecache_groupbox.setVisible(True)
        else:
            self.streaming_groupbox.setVisible(True)
            self.filecache_groupbox.setVisible(False)

    def showAzureModeSettings(self):

        # Azure mode group boxes
        self.accnt_key_groupbox.setVisible(False)
        self.sas_storage_groupbox.setVisible(False)
        self.spn_groupbox.setVisible(False)
        self.msi_groupbox.setVisible(False)

        if self.azModesettings_select.currentIndex() == 0:
            self.accnt_key_groupbox.setVisible(True)
        elif self.azModesettings_select.currentIndex() == 1:
            self.sas_storage_groupbox.setVisible(True)
        elif self.azModesettings_select.currentIndex() == 2:
            self.spn_groupbox.setVisible(True)
        else:
            self.msi_groupbox.setVisible(True)


