# Licensed under the MIT License <http://opensource.org/licenses/MIT>.
#
# Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE

from sys import platform
from PySide6.QtCore import Qt, QSettings
from PySide6 import QtWidgets, QtGui
from PySide6.QtGui import QIcon, QPixmap

# import the custom class made from QtDesigner
from ui_passwordPrompt import Ui_Form

class passwordPrompt(QtWidgets.QMainWindow,Ui_Form):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.myWindow = QSettings('Cloudfuse', 'passwordPrompt')
        #self.initWindowSizePos()
        self.setWindowTitle('File Encrypted')

        ################################################################
        #Template for future reference

        # Hide sensitive data QtWidgets.QLineEdit.EchoMode.PasswordEchoOnEdit
        self.eye_icon = QIcon(QPixmap("hideEye.jpg"))
        self.eye_open_icon= QIcon(QPixmap("openEye.jpg"))
        self.eyeClick = self.lineEdit_password.addAction(self.eye_icon,QtWidgets.QLineEdit.TrailingPosition)
        self.lineEdit_password.setEchoMode(QtWidgets.QLineEdit.EchoMode.Password)
        self.eyeClick.triggered.connect(self.toggleIcon)

        # Set up signals for buttons
        self.button_okay.clicked.connect(self.exitWindow)

    # Set up slots for the signals:

    def toggleIcon(self):
        if self.eyeClick.icon().cacheKey() == self.eye_icon.cacheKey():
            self.eyeClick.setIcon(self.eye_open_icon)
        else:
            self.eyeClick.setIcon(self.eye_icon)

    def exitWindow(self):
        self.close()