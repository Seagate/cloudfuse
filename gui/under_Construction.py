from PySide6.QtWidgets import QWidget, QLabel
from PySide6.QtGui import QPixmap

# import the custom window class
from ui_under_Construction import Ui_Form

class underConstruction(QWidget, Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle("Feature Not available Yet")

        # Close the window when the ok button is clicked
        self.okay_button.clicked.connect(self.close)
        
        # Use a label to pipe in a png picture
        pictureLabel = QLabel(self)
        pixMap = QPixmap("gui/construction.png")
        pictureLabel.setPixmap(pixMap)