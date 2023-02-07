# -*- coding: utf-8 -*-

################################################################################
## Form generated from reading UI file 'MountPrimaryWindow.ui'
##
## Created by: Qt User Interface Compiler version 6.4.2
##
## WARNING! All changes made in this file will be lost when recompiling UI file!
################################################################################

from PySide6.QtCore import (QCoreApplication, QDate, QDateTime, QLocale,
    QMetaObject, QObject, QPoint, QRect,
    QSize, QTime, QUrl, Qt)
from PySide6.QtGui import (QAction, QBrush, QColor, QConicalGradient,
    QCursor, QFont, QFontDatabase, QGradient,
    QIcon, QImage, QKeySequence, QLinearGradient,
    QPainter, QPalette, QPixmap, QRadialGradient,
    QTransform)
from PySide6.QtWidgets import (QApplication, QComboBox, QHBoxLayout, QLabel,
    QLineEdit, QMainWindow, QMenu, QMenuBar,
    QPushButton, QSizePolicy, QStatusBar, QTextEdit,
    QToolButton, QWidget)

class Ui_primaryFUSEwindow(object):
    def setupUi(self, primaryFUSEwindow):
        if not primaryFUSEwindow.objectName():
            primaryFUSEwindow.setObjectName(u"primaryFUSEwindow")
        primaryFUSEwindow.resize(803, 319)
        primaryFUSEwindow.setAnimated(False)
        primaryFUSEwindow.setDocumentMode(False)
        primaryFUSEwindow.setDockOptions(QMainWindow.AllowTabbedDocks)
        self.actionReset_Default_Settings = QAction(primaryFUSEwindow)
        self.actionReset_Default_Settings.setObjectName(u"actionReset_Default_Settings")
        self.advancedSettings_action = QAction(primaryFUSEwindow)
        self.advancedSettings_action.setObjectName(u"advancedSettings_action")
        self.debugLogging_action = QAction(primaryFUSEwindow)
        self.debugLogging_action.setObjectName(u"debugLogging_action")
        self.debugHealthMonitor_action = QAction(primaryFUSEwindow)
        self.debugHealthMonitor_action.setObjectName(u"debugHealthMonitor_action")
        self.debugTesting_action = QAction(primaryFUSEwindow)
        self.debugTesting_action.setObjectName(u"debugTesting_action")
        self.centralwidget = QWidget(primaryFUSEwindow)
        self.centralwidget.setObjectName(u"centralwidget")
        self.mountPoint_input = QLineEdit(self.centralwidget)
        self.mountPoint_input.setObjectName(u"mountPoint_input")
        self.mountPoint_input.setGeometry(QRect(150, 20, 351, 25))
        self.browse_button = QPushButton(self.centralwidget)
        self.browse_button.setObjectName(u"browse_button")
        self.browse_button.setGeometry(QRect(510, 20, 89, 25))
        self.horizontalLayoutWidget = QWidget(self.centralwidget)
        self.horizontalLayoutWidget.setObjectName(u"horizontalLayoutWidget")
        self.horizontalLayoutWidget.setGeometry(QRect(50, 50, 240, 41))
        self.horizontalLayout = QHBoxLayout(self.horizontalLayoutWidget)
        self.horizontalLayout.setObjectName(u"horizontalLayout")
        self.horizontalLayout.setContentsMargins(0, 0, 0, 0)
        self.label_2 = QLabel(self.horizontalLayoutWidget)
        self.label_2.setObjectName(u"label_2")

        self.horizontalLayout.addWidget(self.label_2)

        self.pipeline_select = QComboBox(self.horizontalLayoutWidget)
        self.pipeline_select.addItem("")
        self.pipeline_select.addItem("")
        self.pipeline_select.setObjectName(u"pipeline_select")

        self.horizontalLayout.addWidget(self.pipeline_select)

        self.toolButton = QToolButton(self.centralwidget)
        self.toolButton.setObjectName(u"toolButton")
        self.toolButton.setGeometry(QRect(50, 130, 101, 24))
        self.toolButton.setCheckable(True)
        self.toolButton.setAutoRaise(True)
        self.label = QLabel(self.centralwidget)
        self.label.setObjectName(u"label")
        self.label.setGeometry(QRect(50, 20, 91, 21))
        self.output_textEdit = QTextEdit(self.centralwidget)
        self.output_textEdit.setObjectName(u"output_textEdit")
        self.output_textEdit.setGeometry(QRect(50, 160, 731, 91))
        self.mount_button = QPushButton(self.centralwidget)
        self.mount_button.setObjectName(u"mount_button")
        self.mount_button.setGeometry(QRect(50, 100, 89, 25))
        self.label_3 = QLabel(self.centralwidget)
        self.label_3.setObjectName(u"label_3")
        self.label_3.setGeometry(QRect(180, 100, 481, 111))
        font = QFont()
        font.setPointSize(48)
        font.setBold(True)
        self.label_3.setFont(font)
        self.unmount_button = QPushButton(self.centralwidget)
        self.unmount_button.setObjectName(u"unmount_button")
        self.unmount_button.setGeometry(QRect(150, 100, 89, 25))
        primaryFUSEwindow.setCentralWidget(self.centralwidget)
        self.menubar = QMenuBar(primaryFUSEwindow)
        self.menubar.setObjectName(u"menubar")
        self.menubar.setGeometry(QRect(0, 0, 803, 22))
        self.menuDebug = QMenu(self.menubar)
        self.menuDebug.setObjectName(u"menuDebug")
        self.menuSettings = QMenu(self.menubar)
        self.menuSettings.setObjectName(u"menuSettings")
        primaryFUSEwindow.setMenuBar(self.menubar)
        self.statusbar = QStatusBar(primaryFUSEwindow)
        self.statusbar.setObjectName(u"statusbar")
        primaryFUSEwindow.setStatusBar(self.statusbar)

        self.menubar.addAction(self.menuSettings.menuAction())
        self.menubar.addAction(self.menuDebug.menuAction())
        self.menuDebug.addAction(self.debugLogging_action)
        self.menuDebug.addAction(self.debugHealthMonitor_action)
        self.menuDebug.addAction(self.debugTesting_action)
        self.menuSettings.addAction(self.advancedSettings_action)

        self.retranslateUi(primaryFUSEwindow)

        QMetaObject.connectSlotsByName(primaryFUSEwindow)
    # setupUi

    def retranslateUi(self, primaryFUSEwindow):
        primaryFUSEwindow.setWindowTitle(QCoreApplication.translate("primaryFUSEwindow", u"MainWindow", None))
        self.actionReset_Default_Settings.setText(QCoreApplication.translate("primaryFUSEwindow", u"Reset Default Settings", None))
        self.advancedSettings_action.setText(QCoreApplication.translate("primaryFUSEwindow", u"Advanced", None))
        self.debugLogging_action.setText(QCoreApplication.translate("primaryFUSEwindow", u"Logging", None))
        self.debugHealthMonitor_action.setText(QCoreApplication.translate("primaryFUSEwindow", u"Health Monitor", None))
        self.debugTesting_action.setText(QCoreApplication.translate("primaryFUSEwindow", u"Testing", None))
        self.browse_button.setText(QCoreApplication.translate("primaryFUSEwindow", u"Browse", None))
        self.label_2.setText(QCoreApplication.translate("primaryFUSEwindow", u"Pipeline Selection", None))
        self.pipeline_select.setItemText(0, QCoreApplication.translate("primaryFUSEwindow", u"Streaming", None))
        self.pipeline_select.setItemText(1, QCoreApplication.translate("primaryFUSEwindow", u"File Caching", None))

#if QT_CONFIG(tooltip)
        self.pipeline_select.setToolTip(QCoreApplication.translate("primaryFUSEwindow", u"<html><head/><body><p>Set up the pipeline for LyveFuse. </p><p>Choose streaming or file caching</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.toolButton.setText(QCoreApplication.translate("primaryFUSEwindow", u"Show Progress", None))
        self.label.setText(QCoreApplication.translate("primaryFUSEwindow", u"Mount Point", None))
        self.mount_button.setText(QCoreApplication.translate("primaryFUSEwindow", u"Mount", None))
        self.label_3.setText(QCoreApplication.translate("primaryFUSEwindow", u"ALPHA DESIGN", None))
        self.unmount_button.setText(QCoreApplication.translate("primaryFUSEwindow", u"Unmount", None))
        self.menuDebug.setTitle(QCoreApplication.translate("primaryFUSEwindow", u"Debug", None))
        self.menuSettings.setTitle(QCoreApplication.translate("primaryFUSEwindow", u"Settings", None))
    # retranslateUi

