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

from PySide6.QtWidgets import ( QDialog, QLineEdit, QDialogButtonBox)
from PySide6.QtGui import QIcon, QPixmap
from ui_passwordDialog import Ui_Dialog

class customPasswordDialog(QDialog,Ui_Dialog):
    def __init__(self):
        super().__init__()
        self.setupUi(self)
        self.setWindowTitle('Your Config File Is Encrypted!')

        Qbtn = (QDialogButtonBox.Ok | QDialogButtonBox.Cancel)
        self.buttonBox = QDialogButtonBox(Qbtn)

        self.buttonBox.accepted.connect(self.getPassword)
        self.buttonBox.rejected.connect(self.reject)

        self.buttonBox.clicked.connect(self.getPassword)

        # Hide sensitive data QtWidgets.QLineEdit.EchoMode.PasswordEchoOnEdit
        self.eye_icon = QIcon(QPixmap("gui/hideEye.jpg"))
        self.eye_open_icon= QIcon(QPixmap("gui/openEye.jpg"))
        self.eyeClick = self.lineEdit_password.addAction(self.eye_icon,QLineEdit.TrailingPosition)
        self.lineEdit_password.setEchoMode(QLineEdit.EchoMode.Password)
        self.eyeClick.triggered.connect(self.toggleIcon)

    # Set up slots for the signals:

    def toggleIcon(self):
        if self.eyeClick.icon().cacheKey() == self.eye_icon.cacheKey():
            self.eyeClick.setIcon(self.eye_open_icon)
            self.lineEdit_password.setEchoMode(QLineEdit.EchoMode.Normal)
        else:
            self.eyeClick.setIcon(self.eye_icon)
            self.lineEdit_password.setEchoMode(QLineEdit.EchoMode.Password)

    def getPassword(self):
        return self.lineEdit_password.text()