# Licensed under the MIT License <http://opensource.org/licenses/MIT>.
#
# Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
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

import sys
from PySide6 import QtWidgets

# Main FUSE window
from mountPrimaryWindow import FUSEWindow

# Define the application to run from the Qt library
app = QtWidgets.QApplication(sys.argv)

# Load the main window for the GUI - the very first window the user will see
primaryWindow = FUSEWindow()

# The user won't see a window until we explicitly show it. This can also happen in the FUSE class
#       which will probably happen later down the line, but this is for getting things to work right now
primaryWindow.show()

# Start the app
app.exec()
