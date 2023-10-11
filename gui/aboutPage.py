from PySide6.QtWidgets import QWidget

# import the custom window class
from ui_about_dialog import Ui_About

class aboutPage(QWidget, Ui_About):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        
        # By default, hyperlinks are set to look for local files in the current directory; set the behavior to  open the
        #   external link in the system's default browser
        self.textBrowser.setOpenExternalLinks(True)

        # Close the window when the ok button is clicked
        self.buttonBox.clicked.connect(self.close)
