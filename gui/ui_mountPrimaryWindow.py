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
        self.actionAdvanced = QAction(primaryFUSEwindow)
        self.actionAdvanced.setObjectName(u"actionAdvanced")
        self.actionLogging = QAction(primaryFUSEwindow)
        self.actionLogging.setObjectName(u"actionLogging")
        self.actionHealth_Monitor = QAction(primaryFUSEwindow)
        self.actionHealth_Monitor.setObjectName(u"actionHealth_Monitor")
        self.actionTesting = QAction(primaryFUSEwindow)
        self.actionTesting.setObjectName(u"actionTesting")
        self.centralwidget = QWidget(primaryFUSEwindow)
        self.centralwidget.setObjectName(u"centralwidget")
        self.lineEdit = QLineEdit(self.centralwidget)
        self.lineEdit.setObjectName(u"lineEdit")
        self.lineEdit.setGeometry(QRect(150, 20, 351, 25))
        self.pushButton = QPushButton(self.centralwidget)
        self.pushButton.setObjectName(u"pushButton")
        self.pushButton.setGeometry(QRect(510, 20, 89, 25))
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
        self.textEdit = QTextEdit(self.centralwidget)
        self.textEdit.setObjectName(u"textEdit")
        self.textEdit.setGeometry(QRect(50, 160, 731, 91))
        self.pushButton_2 = QPushButton(self.centralwidget)
        self.pushButton_2.setObjectName(u"pushButton_2")
        self.pushButton_2.setGeometry(QRect(50, 100, 89, 25))
        self.label_3 = QLabel(self.centralwidget)
        self.label_3.setObjectName(u"label_3")
        self.label_3.setGeometry(QRect(180, 100, 481, 111))
        font = QFont()
        font.setPointSize(48)
        font.setBold(True)
        self.label_3.setFont(font)
        primaryFUSEwindow.setCentralWidget(self.centralwidget)
        self.menubar = QMenuBar(primaryFUSEwindow)
        self.menubar.setObjectName(u"menubar")
        self.menubar.setGeometry(QRect(0, 0, 803, 22))
        self.menuSettings = QMenu(self.menubar)
        self.menuSettings.setObjectName(u"menuSettings")
        self.menuDebug = QMenu(self.menubar)
        self.menuDebug.setObjectName(u"menuDebug")
        primaryFUSEwindow.setMenuBar(self.menubar)
        self.statusbar = QStatusBar(primaryFUSEwindow)
        self.statusbar.setObjectName(u"statusbar")
        primaryFUSEwindow.setStatusBar(self.statusbar)

        self.menubar.addAction(self.menuSettings.menuAction())
        self.menubar.addAction(self.menuDebug.menuAction())
        self.menuSettings.addAction(self.actionReset_Default_Settings)
        self.menuSettings.addAction(self.actionAdvanced)
        self.menuDebug.addAction(self.actionLogging)
        self.menuDebug.addAction(self.actionHealth_Monitor)
        self.menuDebug.addAction(self.actionTesting)

        self.retranslateUi(primaryFUSEwindow)

        QMetaObject.connectSlotsByName(primaryFUSEwindow)
    # setupUi

    def retranslateUi(self, primaryFUSEwindow):
        primaryFUSEwindow.setWindowTitle(QCoreApplication.translate("primaryFUSEwindow", u"MainWindow", None))
        self.actionReset_Default_Settings.setText(QCoreApplication.translate("primaryFUSEwindow", u"Reset Default Settings", None))
        self.actionAdvanced.setText(QCoreApplication.translate("primaryFUSEwindow", u"Advanced", None))
        self.actionLogging.setText(QCoreApplication.translate("primaryFUSEwindow", u"Logging", None))
        self.actionHealth_Monitor.setText(QCoreApplication.translate("primaryFUSEwindow", u"Health Monitor", None))
        self.actionTesting.setText(QCoreApplication.translate("primaryFUSEwindow", u"Testing", None))
        self.pushButton.setText(QCoreApplication.translate("primaryFUSEwindow", u"Browse", None))
        self.label_2.setText(QCoreApplication.translate("primaryFUSEwindow", u"Pipeline Selection", None))
        self.pipeline_select.setItemText(0, QCoreApplication.translate("primaryFUSEwindow", u"Streaming", None))
        self.pipeline_select.setItemText(1, QCoreApplication.translate("primaryFUSEwindow", u"File Caching", None))

#if QT_CONFIG(tooltip)
        self.pipeline_select.setToolTip(QCoreApplication.translate("primaryFUSEwindow", u"<html><head/><body><p>Set up the pipeline for LyveFuse. </p><p>Choose streaming or file caching</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.toolButton.setText(QCoreApplication.translate("primaryFUSEwindow", u"Show Progress", None))
        self.label.setText(QCoreApplication.translate("primaryFUSEwindow", u"Mount Point", None))
        self.pushButton_2.setText(QCoreApplication.translate("primaryFUSEwindow", u"Mount", None))
        self.label_3.setText(QCoreApplication.translate("primaryFUSEwindow", u"ALPHA DESIGN", None))
        self.menuSettings.setTitle(QCoreApplication.translate("primaryFUSEwindow", u"Settings", None))
        self.menuDebug.setTitle(QCoreApplication.translate("primaryFUSEwindow", u"Debug", None))
    # retranslateUi

