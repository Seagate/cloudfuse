import sys
from PySide6 import QtWidgets
#import subprocess

# Main FUSE window
from mountPrimaryWindow import FUSEWindow
# Mount settings widget
from mountSettings import mountSettingsWidget

# Define the application to run from the Qt library
app = QtWidgets.QApplication(sys.argv)

# Load the main window for the GUI - the very first window the user will see
primaryWindow = FUSEWindow()

# The user won't see a window until we explicitly show it. This can also happen in the FUSE class
#       which will probably happen later down the line, but this is for getting things to work right now
primaryWindow.show()

# Start the app
app.exec()
