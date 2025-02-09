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

from PySide6.QtWidgets import QWidget
from PySide6.QtGui import QFont

# import the custom window class
from ui_about_dialog import Ui_About

class aboutPage(QWidget, Ui_About):
    def __init__(self,cloudfuseVersion):
        super().__init__()
        self.setupUi(self)

        # By default, hyperlinks are set to look for local files in the current directory; set the behavior to  open the
        #   external link in the system's default browser
        self.textBrowser.setOpenExternalLinks(True)
        self.labelcloudfuseVersion.setText(str(cloudfuseVersion).capitalize())
        self.labelcloudfuseVersion.setFont(QFont('Arial',18,700))

        # Close the window when the ok button is clicked
        self.buttonBox.clicked.connect(self.close)
