# -*- coding: utf-8 -*-

################################################################################
## Form generated from reading UI file 'qt_ui_test.ui'
##
## Created by: Qt User Interface Compiler version 6.4.1
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
from PySide6.QtWidgets import (QApplication, QGridLayout, QGroupBox, QLabel,
    QLineEdit, QMainWindow, QMenu, QMenuBar,
    QPushButton, QSizePolicy, QSpacerItem, QStatusBar,
    QTabWidget, QToolButton, QVBoxLayout, QWidget)

class Ui_MainWindow(object):
    def setupUi(self, MainWindow):
        if not MainWindow.objectName():
            MainWindow.setObjectName(u"MainWindow")
        MainWindow.resize(601, 415)
        MainWindow.setMouseTracking(False)
        self.actionPreferences = QAction(MainWindow)
        self.actionPreferences.setObjectName(u"actionPreferences")
        self.actionAbout_LyveCloudFUSE = QAction(MainWindow)
        self.actionAbout_LyveCloudFUSE.setObjectName(u"actionAbout_LyveCloudFUSE")
        self.centralwidget = QWidget(MainWindow)
        self.centralwidget.setObjectName(u"centralwidget")
        self.centralwidget.setAutoFillBackground(False)
        self.verticalLayout_5 = QVBoxLayout(self.centralwidget)
        self.verticalLayout_5.setObjectName(u"verticalLayout_5")
        self.tabWidget = QTabWidget(self.centralwidget)
        self.tabWidget.setObjectName(u"tabWidget")
        self.tabWidget.setEnabled(True)
        sizePolicy = QSizePolicy(QSizePolicy.Expanding, QSizePolicy.MinimumExpanding)
        sizePolicy.setHorizontalStretch(0)
        sizePolicy.setVerticalStretch(0)
        sizePolicy.setHeightForWidth(self.tabWidget.sizePolicy().hasHeightForWidth())
        self.tabWidget.setSizePolicy(sizePolicy)
        self.tabWidget.setMovable(False)
        self.tab = QWidget()
        self.tab.setObjectName(u"tab")
        self.verticalLayout_4 = QVBoxLayout(self.tab)
        self.verticalLayout_4.setObjectName(u"verticalLayout_4")
        self.groupBox = QGroupBox(self.tab)
        self.groupBox.setObjectName(u"groupBox")
        sizePolicy1 = QSizePolicy(QSizePolicy.Preferred, QSizePolicy.Fixed)
        sizePolicy1.setHorizontalStretch(0)
        sizePolicy1.setVerticalStretch(0)
        sizePolicy1.setHeightForWidth(self.groupBox.sizePolicy().hasHeightForWidth())
        self.groupBox.setSizePolicy(sizePolicy1)
        self.groupBox.setFocusPolicy(Qt.NoFocus)
        self.groupBox.setContextMenuPolicy(Qt.PreventContextMenu)
        self.gridLayout = QGridLayout(self.groupBox)
        self.gridLayout.setObjectName(u"gridLayout")
        self.label = QLabel(self.groupBox)
        self.label.setObjectName(u"label")

        self.gridLayout.addWidget(self.label, 0, 0, 1, 1)

        self.lineEdit = QLineEdit(self.groupBox)
        self.lineEdit.setObjectName(u"lineEdit")

        self.gridLayout.addWidget(self.lineEdit, 0, 1, 1, 1)

        self.toolButton = QToolButton(self.groupBox)
        self.toolButton.setObjectName(u"toolButton")

        self.gridLayout.addWidget(self.toolButton, 0, 2, 1, 1)


        self.verticalLayout_4.addWidget(self.groupBox)

        self.groupBox_2 = QGroupBox(self.tab)
        self.groupBox_2.setObjectName(u"groupBox_2")
        sizePolicy1.setHeightForWidth(self.groupBox_2.sizePolicy().hasHeightForWidth())
        self.groupBox_2.setSizePolicy(sizePolicy1)
        self.groupBox_2.setFocusPolicy(Qt.NoFocus)
        self.groupBox_2.setContextMenuPolicy(Qt.PreventContextMenu)
        self.gridLayout_2 = QGridLayout(self.groupBox_2)
        self.gridLayout_2.setObjectName(u"gridLayout_2")
        self.pushButton = QPushButton(self.groupBox_2)
        self.pushButton.setObjectName(u"pushButton")

        self.gridLayout_2.addWidget(self.pushButton, 0, 0, 1, 1)

        self.pushButton_2 = QPushButton(self.groupBox_2)
        self.pushButton_2.setObjectName(u"pushButton_2")

        self.gridLayout_2.addWidget(self.pushButton_2, 0, 1, 1, 1)


        self.verticalLayout_4.addWidget(self.groupBox_2)

        self.verticalSpacer_General = QSpacerItem(20, 0, QSizePolicy.Minimum, QSizePolicy.Expanding)

        self.verticalLayout_4.addItem(self.verticalSpacer_General)

        self.tabWidget.addTab(self.tab, "")
        self.tab_2 = QWidget()
        self.tab_2.setObjectName(u"tab_2")
        self.tabWidget.addTab(self.tab_2, "")

        self.verticalLayout_5.addWidget(self.tabWidget)

        MainWindow.setCentralWidget(self.centralwidget)
        self.menubar = QMenuBar(MainWindow)
        self.menubar.setObjectName(u"menubar")
        self.menubar.setGeometry(QRect(0, 0, 601, 22))
        self.menuSettings = QMenu(self.menubar)
        self.menuSettings.setObjectName(u"menuSettings")
        self.menuHelp = QMenu(self.menubar)
        self.menuHelp.setObjectName(u"menuHelp")
        MainWindow.setMenuBar(self.menubar)
        self.statusbar = QStatusBar(MainWindow)
        self.statusbar.setObjectName(u"statusbar")
        MainWindow.setStatusBar(self.statusbar)

        self.menubar.addAction(self.menuSettings.menuAction())
        self.menubar.addAction(self.menuHelp.menuAction())
        self.menuSettings.addAction(self.actionPreferences)
        self.menuHelp.addAction(self.actionAbout_LyveCloudFUSE)

        self.retranslateUi(MainWindow)

        self.tabWidget.setCurrentIndex(0)


        QMetaObject.connectSlotsByName(MainWindow)
    # setupUi

    def retranslateUi(self, MainWindow):
        MainWindow.setWindowTitle(QCoreApplication.translate("MainWindow", u"MainWindow", None))
        self.actionPreferences.setText(QCoreApplication.translate("MainWindow", u"Preferences", None))
        self.actionAbout_LyveCloudFUSE.setText(QCoreApplication.translate("MainWindow", u"About LyveCloudFUSE", None))
        self.groupBox.setTitle(QCoreApplication.translate("MainWindow", u"Settings", None))
        self.label.setText(QCoreApplication.translate("MainWindow", u"Mount Folder", None))
        self.toolButton.setText(QCoreApplication.translate("MainWindow", u"Browse", None))
        self.groupBox_2.setTitle(QCoreApplication.translate("MainWindow", u"Mount Options", None))
        self.pushButton.setText(QCoreApplication.translate("MainWindow", u"Mount", None))
        self.pushButton_2.setText(QCoreApplication.translate("MainWindow", u"Unmount", None))
        self.tabWidget.setTabText(self.tabWidget.indexOf(self.tab), QCoreApplication.translate("MainWindow", u"Tab 1", None))
        self.tabWidget.setTabText(self.tabWidget.indexOf(self.tab_2), QCoreApplication.translate("MainWindow", u"Tab 2", None))
        self.menuSettings.setTitle(QCoreApplication.translate("MainWindow", u"Settings", None))
        self.menuHelp.setTitle(QCoreApplication.translate("MainWindow", u"Help", None))
    # retranslateUi

