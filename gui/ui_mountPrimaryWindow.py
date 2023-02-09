# -*- coding: utf-8 -*-

################################################################################
## Form generated from reading UI file 'mountPrimaryWindow.ui'
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
    QWidget)

class Ui_primaryFUSEwindow(object):
    def setupUi(self, primaryFUSEwindow):
        if not primaryFUSEwindow.objectName():
            primaryFUSEwindow.setObjectName(u"primaryFUSEwindow")
        primaryFUSEwindow.resize(755, 331)
        sizePolicy = QSizePolicy(QSizePolicy.Ignored, QSizePolicy.Ignored)
        sizePolicy.setHorizontalStretch(0)
        sizePolicy.setVerticalStretch(0)
        sizePolicy.setHeightForWidth(primaryFUSEwindow.sizePolicy().hasHeightForWidth())
        primaryFUSEwindow.setSizePolicy(sizePolicy)
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
        self.setup_action = QAction(primaryFUSEwindow)
        self.setup_action.setObjectName(u"setup_action")
        self.centralwidget = QWidget(primaryFUSEwindow)
        self.centralwidget.setObjectName(u"centralwidget")
        self.horizontalLayoutWidget = QWidget(self.centralwidget)
        self.horizontalLayoutWidget.setObjectName(u"horizontalLayoutWidget")
        self.horizontalLayoutWidget.setGeometry(QRect(10, 60, 240, 41))
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

        self.output_textEdit = QTextEdit(self.centralwidget)
        self.output_textEdit.setObjectName(u"output_textEdit")
        self.output_textEdit.setGeometry(QRect(10, 180, 731, 91))
        sizePolicy1 = QSizePolicy(QSizePolicy.MinimumExpanding, QSizePolicy.MinimumExpanding)
        sizePolicy1.setHorizontalStretch(0)
        sizePolicy1.setVerticalStretch(0)
        sizePolicy1.setHeightForWidth(self.output_textEdit.sizePolicy().hasHeightForWidth())
        self.output_textEdit.setSizePolicy(sizePolicy1)
        self.mount_button = QPushButton(self.centralwidget)
        self.mount_button.setObjectName(u"mount_button")
        self.mount_button.setGeometry(QRect(10, 110, 89, 25))
        self.label_3 = QLabel(self.centralwidget)
        self.label_3.setObjectName(u"label_3")
        self.label_3.setGeometry(QRect(180, 130, 391, 61))
        font = QFont()
        font.setFamilies([u"Ubuntu"])
        font.setPointSize(28)
        font.setBold(True)
        font.setStrikeOut(False)
        font.setKerning(True)
        self.label_3.setFont(font)
        self.label_3.setTextFormat(Qt.RichText)
        self.label_3.setAlignment(Qt.AlignCenter)
        self.unmount_button = QPushButton(self.centralwidget)
        self.unmount_button.setObjectName(u"unmount_button")
        self.unmount_button.setGeometry(QRect(110, 110, 89, 25))
        self.horizontalLayoutWidget_2 = QWidget(self.centralwidget)
        self.horizontalLayoutWidget_2.setObjectName(u"horizontalLayoutWidget_2")
        self.horizontalLayoutWidget_2.setGeometry(QRect(260, 60, 212, 41))
        self.horizontalLayout_2 = QHBoxLayout(self.horizontalLayoutWidget_2)
        self.horizontalLayout_2.setObjectName(u"horizontalLayout_2")
        self.horizontalLayout_2.setContentsMargins(0, 0, 0, 0)
        self.label_4 = QLabel(self.horizontalLayoutWidget_2)
        self.label_4.setObjectName(u"label_4")

        self.horizontalLayout_2.addWidget(self.label_4)

        self.bucket_select = QComboBox(self.horizontalLayoutWidget_2)
        self.bucket_select.addItem("")
        self.bucket_select.addItem("")
        self.bucket_select.addItem("")
        self.bucket_select.setObjectName(u"bucket_select")

        self.horizontalLayout_2.addWidget(self.bucket_select)

        self.horizontalLayoutWidget_3 = QWidget(self.centralwidget)
        self.horizontalLayoutWidget_3.setObjectName(u"horizontalLayoutWidget_3")
        self.horizontalLayoutWidget_3.setGeometry(QRect(10, 10, 421, 51))
        self.horizontalLayout_3 = QHBoxLayout(self.horizontalLayoutWidget_3)
        self.horizontalLayout_3.setObjectName(u"horizontalLayout_3")
        self.horizontalLayout_3.setContentsMargins(0, 0, 0, 0)
        self.label = QLabel(self.horizontalLayoutWidget_3)
        self.label.setObjectName(u"label")

        self.horizontalLayout_3.addWidget(self.label)

        self.mountPoint_input = QLineEdit(self.horizontalLayoutWidget_3)
        self.mountPoint_input.setObjectName(u"mountPoint_input")

        self.horizontalLayout_3.addWidget(self.mountPoint_input)

        self.browse_button = QPushButton(self.horizontalLayoutWidget_3)
        self.browse_button.setObjectName(u"browse_button")

        self.horizontalLayout_3.addWidget(self.browse_button)

        primaryFUSEwindow.setCentralWidget(self.centralwidget)
        self.menubar = QMenuBar(primaryFUSEwindow)
        self.menubar.setObjectName(u"menubar")
        self.menubar.setGeometry(QRect(0, 0, 755, 22))
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
        self.menuSettings.addAction(self.setup_action)
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
        self.setup_action.setText(QCoreApplication.translate("primaryFUSEwindow", u"Setup", None))
        self.label_2.setText(QCoreApplication.translate("primaryFUSEwindow", u"Pipeline Selection", None))
        self.pipeline_select.setItemText(0, QCoreApplication.translate("primaryFUSEwindow", u"Streaming", None))
        self.pipeline_select.setItemText(1, QCoreApplication.translate("primaryFUSEwindow", u"File Caching", None))

#if QT_CONFIG(tooltip)
        self.pipeline_select.setToolTip(QCoreApplication.translate("primaryFUSEwindow", u"<html><head/><body><p>Set up the pipeline for LyveFuse. </p><p>Choose streaming or file caching</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.mount_button.setText(QCoreApplication.translate("primaryFUSEwindow", u"Mount", None))
        self.label_3.setText(QCoreApplication.translate("primaryFUSEwindow", u"ALPHA DESIGN", None))
        self.unmount_button.setText(QCoreApplication.translate("primaryFUSEwindow", u"Unmount", None))
        self.label_4.setText(QCoreApplication.translate("primaryFUSEwindow", u"Mount Target", None))
        self.bucket_select.setItemText(0, QCoreApplication.translate("primaryFUSEwindow", u"Lyve", None))
        self.bucket_select.setItemText(1, QCoreApplication.translate("primaryFUSEwindow", u"Azure", None))
        self.bucket_select.setItemText(2, QCoreApplication.translate("primaryFUSEwindow", u"S3", None))

#if QT_CONFIG(tooltip)
        self.bucket_select.setToolTip(QCoreApplication.translate("primaryFUSEwindow", u"<html><head/><body><p>Set up the pipeline for LyveFuse. </p><p>Choose streaming or file caching</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.bucket_select.setCurrentText(QCoreApplication.translate("primaryFUSEwindow", u"Lyve", None))
        self.label.setText(QCoreApplication.translate("primaryFUSEwindow", u"Mount Point", None))
        self.browse_button.setText(QCoreApplication.translate("primaryFUSEwindow", u"Browse", None))
        self.menuDebug.setTitle(QCoreApplication.translate("primaryFUSEwindow", u"Debug", None))
        self.menuSettings.setTitle(QCoreApplication.translate("primaryFUSEwindow", u"Settings", None))
    # retranslateUi

