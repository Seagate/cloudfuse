import sys
from PySide6 import QtWidgets

import subprocess

from sample_window import Ui_MainWindow


class MainWindow(QtWidgets.QMainWindow, Ui_MainWindow):
  def __init__(self):
    super(MainWindow, self).__init__()
    self.setupUi(self)

    self.toolButton.clicked.connect(self.openFileDialog)

    self.pushButton.clicked.connect(self.onMountClick)
    self.pushButton_2.clicked.connect(self.onUnmountClick)
  
  def openFileDialog(self):
    directory = str(QtWidgets.QFileDialog.getExistingDirectory())
    self.lineEdit.setText('{}'.format(directory))

  def onMountClick(self):
    try:
      mount_directory = str(self.lineEdit.text())
      print(mount_directory)

      mount = subprocess.run(["./azure-storage-fuse", "mount", "all", mount_directory, "--config-file=./config.yaml"])
      if mount.returncode == 0:
        msg = QtWidgets.QMessageBox()
        msg.setWindowTitle("Success")
        msg.setText("Successfully mounted container")
        x = msg.exec()  # this will show our messagebox

      else:
        msg = QtWidgets.QMessageBox()
        msg.setWindowTitle("Error")
        msg.setText("Error mounting container")
        x = msg.exec()  # this will show our messagebox

    except ValueError:
      pass

  def onUnmountClick(self):
    try:
      umount = subprocess.run(["./azure-storage-fuse", "unmount", "all"])
      if umount.returncode == 0:
        msg = QtWidgets.QMessageBox()
        msg.setWindowTitle("Success")
        msg.setText("Successfully unmounted container")
        x = msg.exec()  # this will show our messagebox
      else:
        msg = QtWidgets.QMessageBox()
        msg.setWindowTitle("Error")
        msg.setText("Error unmounting container")
        x = msg.exec()  # this will show our messagebox

    except ValueError:
      pass


app = QtWidgets.QApplication(sys.argv)

window = MainWindow()
window.show()
app.exec()
