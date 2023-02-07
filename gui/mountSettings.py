from PySide6.QtCore import Qt
from PySide6.QtWidgets import QWidget

# import the custom class made from QtDesigner
from ui_mountSettings import Ui_mountSettingsWidget

class mountSettingsWidget(QWidget, Ui_mountSettingsWidget):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        # Set the title of the widget window
        self.setWindowTitle("Advanced Settings")
        
        # set up the signals to be activated

        # Example - button.clicked.connect() 
        #           button = the thing being interacted with
        #           clicked = one of the actions available to trigger the signal
        #           connect = activate the signal for slots to be triggered

        self.resetDefaultSettings_checkbox.stateChanged.connect(self.do_something)
        self.fileCache_path_input.editingFinished.connect(self.fileCache_input)
    
    # Set up slots

    # Placeholder for an actual slot to be used
    def do_something(self):
        print('Default changed')

    # At the moment, prints the user input to show 'hello world' level of QlineEdit
    def fileCache_input(self):
        print(self.fileCache_path_input.text())