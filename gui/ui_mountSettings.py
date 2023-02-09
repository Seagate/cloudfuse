# -*- coding: utf-8 -*-

################################################################################
## Form generated from reading UI file 'mountSettings.ui'
##
## Created by: Qt User Interface Compiler version 6.4.2
##
## WARNING! All changes made in this file will be lost when recompiling UI file!
################################################################################

from PySide6.QtCore import (QCoreApplication, QDate, QDateTime, QLocale,
    QMetaObject, QObject, QPoint, QRect,
    QSize, QTime, QUrl, Qt)
from PySide6.QtGui import (QBrush, QColor, QConicalGradient, QCursor,
    QFont, QFontDatabase, QGradient, QIcon,
    QImage, QKeySequence, QLinearGradient, QPainter,
    QPalette, QPixmap, QRadialGradient, QTransform)
from PySide6.QtWidgets import (QApplication, QCheckBox, QComboBox, QGridLayout,
    QGroupBox, QHBoxLayout, QLabel, QLineEdit,
    QPushButton, QSizePolicy, QVBoxLayout, QWidget)

class Ui_mountSettingsWidget(object):
    def setupUi(self, mountSettingsWidget):
        if not mountSettingsWidget.objectName():
            mountSettingsWidget.setObjectName(u"mountSettingsWidget")
        mountSettingsWidget.resize(1770, 701)
        self.libfuse_groupBox = QGroupBox(mountSettingsWidget)
        self.libfuse_groupBox.setObjectName(u"libfuse_groupBox")
        self.libfuse_groupBox.setGeometry(QRect(10, 140, 401, 291))
        self.libfuse_groupBox.setLayoutDirection(Qt.LeftToRight)
        self.libfuse_groupBox.setAlignment(Qt.AlignCenter)
        self.libfuse_groupBox.setFlat(False)
        self.libfuse_groupBox.setCheckable(False)
        self.libfuse_groupBox.setChecked(False)
        self.verticalLayoutWidget_3 = QWidget(self.libfuse_groupBox)
        self.verticalLayoutWidget_3.setObjectName(u"verticalLayoutWidget_3")
        self.verticalLayoutWidget_3.setGeometry(QRect(10, 30, 383, 250))
        self.verticalLayout_3 = QVBoxLayout(self.verticalLayoutWidget_3)
        self.verticalLayout_3.setObjectName(u"verticalLayout_3")
        self.verticalLayout_3.setContentsMargins(0, 0, 0, 0)
        self.horizontalLayout_2 = QHBoxLayout()
        self.horizontalLayout_2.setObjectName(u"horizontalLayout_2")
        self.label_3 = QLabel(self.verticalLayoutWidget_3)
        self.label_3.setObjectName(u"label_3")

        self.horizontalLayout_2.addWidget(self.label_3)

        self.libfuse_permissions_select = QComboBox(self.verticalLayoutWidget_3)
        self.libfuse_permissions_select.addItem("")
        self.libfuse_permissions_select.addItem("")
        self.libfuse_permissions_select.addItem("")
        self.libfuse_permissions_select.addItem("")
        self.libfuse_permissions_select.setObjectName(u"libfuse_permissions_select")

        self.horizontalLayout_2.addWidget(self.libfuse_permissions_select)


        self.verticalLayout_3.addLayout(self.horizontalLayout_2)

        self.horizontalLayout_4 = QHBoxLayout()
        self.horizontalLayout_4.setObjectName(u"horizontalLayout_4")
        self.verticalLayout = QVBoxLayout()
        self.verticalLayout.setObjectName(u"verticalLayout")
        self.label_4 = QLabel(self.verticalLayoutWidget_3)
        self.label_4.setObjectName(u"label_4")

        self.verticalLayout.addWidget(self.label_4)

        self.label_5 = QLabel(self.verticalLayoutWidget_3)
        self.label_5.setObjectName(u"label_5")

        self.verticalLayout.addWidget(self.label_5)

        self.label_6 = QLabel(self.verticalLayoutWidget_3)
        self.label_6.setObjectName(u"label_6")

        self.verticalLayout.addWidget(self.label_6)


        self.horizontalLayout_4.addLayout(self.verticalLayout)

        self.verticalLayout_2 = QVBoxLayout()
        self.verticalLayout_2.setObjectName(u"verticalLayout_2")
        self.libfuse_attExp_input = QLineEdit(self.verticalLayoutWidget_3)
        self.libfuse_attExp_input.setObjectName(u"libfuse_attExp_input")
        self.libfuse_attExp_input.setMaximumSize(QSize(30, 16777215))

        self.verticalLayout_2.addWidget(self.libfuse_attExp_input)

        self.libfuse_entExp_input = QLineEdit(self.verticalLayoutWidget_3)
        self.libfuse_entExp_input.setObjectName(u"libfuse_entExp_input")
        self.libfuse_entExp_input.setMaximumSize(QSize(30, 16777215))

        self.verticalLayout_2.addWidget(self.libfuse_entExp_input)

        self.libfuse_pathExp_input = QLineEdit(self.verticalLayoutWidget_3)
        self.libfuse_pathExp_input.setObjectName(u"libfuse_pathExp_input")
        self.libfuse_pathExp_input.setMaximumSize(QSize(30, 16777215))

        self.verticalLayout_2.addWidget(self.libfuse_pathExp_input)


        self.horizontalLayout_4.addLayout(self.verticalLayout_2)


        self.verticalLayout_3.addLayout(self.horizontalLayout_4)

        self.libfuse_disableWriteback_checkbox = QCheckBox(self.verticalLayoutWidget_3)
        self.libfuse_disableWriteback_checkbox.setObjectName(u"libfuse_disableWriteback_checkbox")

        self.verticalLayout_3.addWidget(self.libfuse_disableWriteback_checkbox)

        self.libfuse_ignoreAppend_checkbox = QCheckBox(self.verticalLayoutWidget_3)
        self.libfuse_ignoreAppend_checkbox.setObjectName(u"libfuse_ignoreAppend_checkbox")

        self.verticalLayout_3.addWidget(self.libfuse_ignoreAppend_checkbox)

        self.streaming_groupBox = QGroupBox(mountSettingsWidget)
        self.streaming_groupBox.setObjectName(u"streaming_groupBox")
        self.streaming_groupBox.setEnabled(True)
        self.streaming_groupBox.setGeometry(QRect(10, 440, 351, 221))
        font = QFont()
        font.setPointSize(11)
        self.streaming_groupBox.setFont(font)
        self.streaming_groupBox.setCursor(QCursor(Qt.ArrowCursor))
        self.streaming_groupBox.setAlignment(Qt.AlignCenter)
        self.streaming_groupBox.setFlat(False)
        self.streaming_groupBox.setCheckable(False)
        self.streaming_groupBox.setChecked(False)
        self.verticalLayoutWidget_6 = QWidget(self.streaming_groupBox)
        self.verticalLayoutWidget_6.setObjectName(u"verticalLayoutWidget_6")
        self.verticalLayoutWidget_6.setGeometry(QRect(10, 30, 333, 204))
        self.verticalLayout_6 = QVBoxLayout(self.verticalLayoutWidget_6)
        self.verticalLayout_6.setObjectName(u"verticalLayout_6")
        self.verticalLayout_6.setContentsMargins(0, 0, 0, 0)
        self.horizontalLayout_3 = QHBoxLayout()
        self.horizontalLayout_3.setObjectName(u"horizontalLayout_3")
        self.verticalLayout_4 = QVBoxLayout()
        self.verticalLayout_4.setObjectName(u"verticalLayout_4")
        self.label_8 = QLabel(self.verticalLayoutWidget_6)
        self.label_8.setObjectName(u"label_8")

        self.verticalLayout_4.addWidget(self.label_8)

        self.label_9 = QLabel(self.verticalLayoutWidget_6)
        self.label_9.setObjectName(u"label_9")

        self.verticalLayout_4.addWidget(self.label_9)

        self.label_10 = QLabel(self.verticalLayoutWidget_6)
        self.label_10.setObjectName(u"label_10")

        self.verticalLayout_4.addWidget(self.label_10)


        self.horizontalLayout_3.addLayout(self.verticalLayout_4)

        self.verticalLayout_5 = QVBoxLayout()
        self.verticalLayout_5.setObjectName(u"verticalLayout_5")
        self.streaming_blksz_input = QLineEdit(self.verticalLayoutWidget_6)
        self.streaming_blksz_input.setObjectName(u"streaming_blksz_input")
        self.streaming_blksz_input.setMaximumSize(QSize(75, 16777215))
        self.streaming_blksz_input.setAlignment(Qt.AlignCenter)

        self.verticalLayout_5.addWidget(self.streaming_blksz_input)

        self.streaming_mxbuff_input = QLineEdit(self.verticalLayoutWidget_6)
        self.streaming_mxbuff_input.setObjectName(u"streaming_mxbuff_input")
        self.streaming_mxbuff_input.setMaximumSize(QSize(75, 16777215))
        self.streaming_mxbuff_input.setAlignment(Qt.AlignCenter)

        self.verticalLayout_5.addWidget(self.streaming_mxbuff_input)

        self.streaming_buffsz_input = QLineEdit(self.verticalLayoutWidget_6)
        self.streaming_buffsz_input.setObjectName(u"streaming_buffsz_input")
        self.streaming_buffsz_input.setMaximumSize(QSize(75, 16777215))
        self.streaming_buffsz_input.setAlignment(Qt.AlignCenter)

        self.verticalLayout_5.addWidget(self.streaming_buffsz_input)


        self.horizontalLayout_3.addLayout(self.verticalLayout_5)


        self.verticalLayout_6.addLayout(self.horizontalLayout_3)

        self.horizontalLayout_5 = QHBoxLayout()
        self.horizontalLayout_5.setObjectName(u"horizontalLayout_5")
        self.label_11 = QLabel(self.verticalLayoutWidget_6)
        self.label_11.setObjectName(u"label_11")

        self.horizontalLayout_5.addWidget(self.label_11)

        self.streaming_fileCachingLevel_select = QComboBox(self.verticalLayoutWidget_6)
        self.streaming_fileCachingLevel_select.addItem("")
        self.streaming_fileCachingLevel_select.addItem("")
        self.streaming_fileCachingLevel_select.setObjectName(u"streaming_fileCachingLevel_select")

        self.horizontalLayout_5.addWidget(self.streaming_fileCachingLevel_select)


        self.verticalLayout_6.addLayout(self.horizontalLayout_5)

        self.fileCache_groupBox = QGroupBox(mountSettingsWidget)
        self.fileCache_groupBox.setObjectName(u"fileCache_groupBox")
        self.fileCache_groupBox.setGeometry(QRect(430, 10, 441, 441))
        self.fileCache_groupBox.setAlignment(Qt.AlignCenter)
        self.fileCacheAdvancedSettings_groupBox = QGroupBox(self.fileCache_groupBox)
        self.fileCacheAdvancedSettings_groupBox.setObjectName(u"fileCacheAdvancedSettings_groupBox")
        self.fileCacheAdvancedSettings_groupBox.setGeometry(QRect(10, 60, 421, 371))
        self.fileCacheAdvancedSettings_groupBox.setCheckable(True)
        self.fileCacheAdvancedSettings_groupBox.setChecked(False)
        self.verticalLayoutWidget_2 = QWidget(self.fileCacheAdvancedSettings_groupBox)
        self.verticalLayoutWidget_2.setObjectName(u"verticalLayoutWidget_2")
        self.verticalLayoutWidget_2.setGeometry(QRect(10, 30, 396, 331))
        self.verticalLayout_9 = QVBoxLayout(self.verticalLayoutWidget_2)
        self.verticalLayout_9.setObjectName(u"verticalLayout_9")
        self.verticalLayout_9.setContentsMargins(0, 0, 0, 0)
        self.horizontalLayout_7 = QHBoxLayout()
        self.horizontalLayout_7.setObjectName(u"horizontalLayout_7")
        self.verticalLayout_8 = QVBoxLayout()
        self.verticalLayout_8.setObjectName(u"verticalLayout_8")
        self.label_14 = QLabel(self.verticalLayoutWidget_2)
        self.label_14.setObjectName(u"label_14")

        self.verticalLayout_8.addWidget(self.label_14)

        self.label_15 = QLabel(self.verticalLayoutWidget_2)
        self.label_15.setObjectName(u"label_15")

        self.verticalLayout_8.addWidget(self.label_15)

        self.label_16 = QLabel(self.verticalLayoutWidget_2)
        self.label_16.setObjectName(u"label_16")

        self.verticalLayout_8.addWidget(self.label_16)

        self.label_17 = QLabel(self.verticalLayoutWidget_2)
        self.label_17.setObjectName(u"label_17")

        self.verticalLayout_8.addWidget(self.label_17)

        self.label_18 = QLabel(self.verticalLayoutWidget_2)
        self.label_18.setObjectName(u"label_18")

        self.verticalLayout_8.addWidget(self.label_18)

        self.label_19 = QLabel(self.verticalLayoutWidget_2)
        self.label_19.setObjectName(u"label_19")

        self.verticalLayout_8.addWidget(self.label_19)


        self.horizontalLayout_7.addLayout(self.verticalLayout_8)

        self.verticalLayout_7 = QVBoxLayout()
        self.verticalLayout_7.setObjectName(u"verticalLayout_7")
        self.fileCache_evictionPolicy_select = QComboBox(self.verticalLayoutWidget_2)
        self.fileCache_evictionPolicy_select.addItem("")
        self.fileCache_evictionPolicy_select.addItem("")
        self.fileCache_evictionPolicy_select.setObjectName(u"fileCache_evictionPolicy_select")

        self.verticalLayout_7.addWidget(self.fileCache_evictionPolicy_select)

        self.fileCache_timeout_input = QLineEdit(self.verticalLayoutWidget_2)
        self.fileCache_timeout_input.setObjectName(u"fileCache_timeout_input")
        self.fileCache_timeout_input.setMaximumSize(QSize(30, 16777215))
        self.fileCache_timeout_input.setAlignment(Qt.AlignCenter)

        self.verticalLayout_7.addWidget(self.fileCache_timeout_input)

        self.fileCache_maxEviction_input = QLineEdit(self.verticalLayoutWidget_2)
        self.fileCache_maxEviction_input.setObjectName(u"fileCache_maxEviction_input")
        self.fileCache_maxEviction_input.setMaximumSize(QSize(75, 16777215))
        self.fileCache_maxEviction_input.setAlignment(Qt.AlignCenter)

        self.verticalLayout_7.addWidget(self.fileCache_maxEviction_input)

        self.fileCache_maxCacheSize_input = QLineEdit(self.verticalLayoutWidget_2)
        self.fileCache_maxCacheSize_input.setObjectName(u"fileCache_maxCacheSize_input")
        self.fileCache_maxCacheSize_input.setMaximumSize(QSize(75, 16777215))
        self.fileCache_maxCacheSize_input.setAlignment(Qt.AlignCenter)

        self.verticalLayout_7.addWidget(self.fileCache_maxCacheSize_input)

        self.fileCache_evictMaxThresh_input = QLineEdit(self.verticalLayoutWidget_2)
        self.fileCache_evictMaxThresh_input.setObjectName(u"fileCache_evictMaxThresh_input")
        self.fileCache_evictMaxThresh_input.setMaximumSize(QSize(30, 16777215))
        self.fileCache_evictMaxThresh_input.setAlignment(Qt.AlignCenter)

        self.verticalLayout_7.addWidget(self.fileCache_evictMaxThresh_input)

        self.fileCache_evictMinThresh_input = QLineEdit(self.verticalLayoutWidget_2)
        self.fileCache_evictMinThresh_input.setObjectName(u"fileCache_evictMinThresh_input")
        self.fileCache_evictMinThresh_input.setMaximumSize(QSize(30, 16777215))
        self.fileCache_evictMinThresh_input.setAlignment(Qt.AlignCenter)

        self.verticalLayout_7.addWidget(self.fileCache_evictMinThresh_input)


        self.horizontalLayout_7.addLayout(self.verticalLayout_7)


        self.verticalLayout_9.addLayout(self.horizontalLayout_7)

        self.fileCache_allowNonEmptyTmp_checkbox = QCheckBox(self.verticalLayoutWidget_2)
        self.fileCache_allowNonEmptyTmp_checkbox.setObjectName(u"fileCache_allowNonEmptyTmp_checkbox")

        self.verticalLayout_9.addWidget(self.fileCache_allowNonEmptyTmp_checkbox)

        self.fileCache_policyLogs_checkbox = QCheckBox(self.verticalLayoutWidget_2)
        self.fileCache_policyLogs_checkbox.setObjectName(u"fileCache_policyLogs_checkbox")

        self.verticalLayout_9.addWidget(self.fileCache_policyLogs_checkbox)

        self.fileCache_createEmptyFile_checkbox = QCheckBox(self.verticalLayoutWidget_2)
        self.fileCache_createEmptyFile_checkbox.setObjectName(u"fileCache_createEmptyFile_checkbox")

        self.verticalLayout_9.addWidget(self.fileCache_createEmptyFile_checkbox)

        self.fileCache_cleanupStart_checkbox = QCheckBox(self.verticalLayoutWidget_2)
        self.fileCache_cleanupStart_checkbox.setObjectName(u"fileCache_cleanupStart_checkbox")

        self.verticalLayout_9.addWidget(self.fileCache_cleanupStart_checkbox)

        self.fileCache_offloadIO_checkbox = QCheckBox(self.verticalLayoutWidget_2)
        self.fileCache_offloadIO_checkbox.setObjectName(u"fileCache_offloadIO_checkbox")

        self.verticalLayout_9.addWidget(self.fileCache_offloadIO_checkbox)

        self.horizontalLayoutWidget_6 = QWidget(self.fileCache_groupBox)
        self.horizontalLayoutWidget_6.setObjectName(u"horizontalLayoutWidget_6")
        self.horizontalLayoutWidget_6.setGeometry(QRect(0, 20, 331, 38))
        self.horizontalLayout_6 = QHBoxLayout(self.horizontalLayoutWidget_6)
        self.horizontalLayout_6.setObjectName(u"horizontalLayout_6")
        self.horizontalLayout_6.setContentsMargins(0, 0, 0, 0)
        self.label_13 = QLabel(self.horizontalLayoutWidget_6)
        self.label_13.setObjectName(u"label_13")

        self.horizontalLayout_6.addWidget(self.label_13)

        self.fileCache_path_input = QLineEdit(self.horizontalLayoutWidget_6)
        self.fileCache_path_input.setObjectName(u"fileCache_path_input")

        self.horizontalLayout_6.addWidget(self.fileCache_path_input)

        self.verticalLayoutWidget_4 = QWidget(mountSettingsWidget)
        self.verticalLayoutWidget_4.setObjectName(u"verticalLayoutWidget_4")
        self.verticalLayoutWidget_4.setGeometry(QRect(10, 10, 218, 112))
        self.verticalLayout_10 = QVBoxLayout(self.verticalLayoutWidget_4)
        self.verticalLayout_10.setObjectName(u"verticalLayout_10")
        self.verticalLayout_10.setContentsMargins(0, 0, 0, 0)
        self.commonConfig_multiuser_checkbox = QCheckBox(self.verticalLayoutWidget_4)
        self.commonConfig_multiuser_checkbox.setObjectName(u"commonConfig_multiuser_checkbox")

        self.verticalLayout_10.addWidget(self.commonConfig_multiuser_checkbox)

        self.commonConfig_nonEmptyDir_checkbox = QCheckBox(self.verticalLayoutWidget_4)
        self.commonConfig_nonEmptyDir_checkbox.setObjectName(u"commonConfig_nonEmptyDir_checkbox")

        self.verticalLayout_10.addWidget(self.commonConfig_nonEmptyDir_checkbox)

        self.commonConfig_readOnly_checkbox = QCheckBox(self.verticalLayoutWidget_4)
        self.commonConfig_readOnly_checkbox.setObjectName(u"commonConfig_readOnly_checkbox")

        self.verticalLayout_10.addWidget(self.commonConfig_readOnly_checkbox)

        self.daemonForeground_checkbox = QCheckBox(self.verticalLayoutWidget_4)
        self.daemonForeground_checkbox.setObjectName(u"daemonForeground_checkbox")

        self.verticalLayout_10.addWidget(self.daemonForeground_checkbox)

        self.label = QLabel(mountSettingsWidget)
        self.label.setObjectName(u"label")
        self.label.setGeometry(QRect(310, 250, 481, 181))
        font1 = QFont()
        font1.setPointSize(48)
        font1.setBold(True)
        self.label.setFont(font1)
        self.pushButton = QPushButton(mountSettingsWidget)
        self.pushButton.setObjectName(u"pushButton")
        self.pushButton.setGeometry(QRect(10, 670, 161, 25))
        self.AzureBucket_groupbox = QGroupBox(mountSettingsWidget)
        self.AzureBucket_groupbox.setObjectName(u"AzureBucket_groupbox")
        self.AzureBucket_groupbox.setGeometry(QRect(880, 10, 881, 471))
        self.AzureBucket_groupbox.setAlignment(Qt.AlignCenter)
        self.verticalLayoutWidget_8 = QWidget(self.AzureBucket_groupbox)
        self.verticalLayoutWidget_8.setObjectName(u"verticalLayoutWidget_8")
        self.verticalLayoutWidget_8.setGeometry(QRect(10, 30, 318, 367))
        self.verticalLayout_14 = QVBoxLayout(self.verticalLayoutWidget_8)
        self.verticalLayout_14.setObjectName(u"verticalLayout_14")
        self.verticalLayout_14.setContentsMargins(0, 0, 0, 0)
        self.verticalLayout_11 = QVBoxLayout()
        self.verticalLayout_11.setObjectName(u"verticalLayout_11")
        self.horizontalLayout_9 = QHBoxLayout()
        self.horizontalLayout_9.setObjectName(u"horizontalLayout_9")
        self.label_2 = QLabel(self.verticalLayoutWidget_8)
        self.label_2.setObjectName(u"label_2")

        self.horizontalLayout_9.addWidget(self.label_2)

        self.AzTypesettings_select = QComboBox(self.verticalLayoutWidget_8)
        self.AzTypesettings_select.addItem("")
        self.AzTypesettings_select.addItem("")
        self.AzTypesettings_select.setObjectName(u"AzTypesettings_select")

        self.horizontalLayout_9.addWidget(self.AzTypesettings_select)


        self.verticalLayout_11.addLayout(self.horizontalLayout_9)

        self.horizontalLayout = QHBoxLayout()
        self.horizontalLayout.setObjectName(u"horizontalLayout")
        self.label_7 = QLabel(self.verticalLayoutWidget_8)
        self.label_7.setObjectName(u"label_7")

        self.horizontalLayout.addWidget(self.label_7)

        self.azAccountName_input = QLineEdit(self.verticalLayoutWidget_8)
        self.azAccountName_input.setObjectName(u"azAccountName_input")

        self.horizontalLayout.addWidget(self.azAccountName_input)


        self.verticalLayout_11.addLayout(self.horizontalLayout)

        self.horizontalLayout_8 = QHBoxLayout()
        self.horizontalLayout_8.setObjectName(u"horizontalLayout_8")
        self.label_12 = QLabel(self.verticalLayoutWidget_8)
        self.label_12.setObjectName(u"label_12")

        self.horizontalLayout_8.addWidget(self.label_12)

        self.azContainer_input = QLineEdit(self.verticalLayoutWidget_8)
        self.azContainer_input.setObjectName(u"azContainer_input")

        self.horizontalLayout_8.addWidget(self.azContainer_input)


        self.verticalLayout_11.addLayout(self.horizontalLayout_8)

        self.horizontalLayout_10 = QHBoxLayout()
        self.horizontalLayout_10.setObjectName(u"horizontalLayout_10")
        self.label_20 = QLabel(self.verticalLayoutWidget_8)
        self.label_20.setObjectName(u"label_20")

        self.horizontalLayout_10.addWidget(self.label_20)

        self.azEndpoint_input = QLineEdit(self.verticalLayoutWidget_8)
        self.azEndpoint_input.setObjectName(u"azEndpoint_input")

        self.horizontalLayout_10.addWidget(self.azEndpoint_input)


        self.verticalLayout_11.addLayout(self.horizontalLayout_10)

        self.horizontalLayout_11 = QHBoxLayout()
        self.horizontalLayout_11.setObjectName(u"horizontalLayout_11")
        self.label_21 = QLabel(self.verticalLayoutWidget_8)
        self.label_21.setObjectName(u"label_21")

        self.horizontalLayout_11.addWidget(self.label_21)

        self.azModesettings_select = QComboBox(self.verticalLayoutWidget_8)
        self.azModesettings_select.addItem("")
        self.azModesettings_select.addItem("")
        self.azModesettings_select.addItem("")
        self.azModesettings_select.addItem("")
        self.azModesettings_select.setObjectName(u"azModesettings_select")

        self.horizontalLayout_11.addWidget(self.azModesettings_select)


        self.verticalLayout_11.addLayout(self.horizontalLayout_11)


        self.verticalLayout_14.addLayout(self.verticalLayout_11)

        self.gridLayout = QGridLayout()
        self.gridLayout.setObjectName(u"gridLayout")
        self.horizontalLayout_12 = QHBoxLayout()
        self.horizontalLayout_12.setObjectName(u"horizontalLayout_12")
        self.label_22 = QLabel(self.verticalLayoutWidget_8)
        self.label_22.setObjectName(u"label_22")

        self.horizontalLayout_12.addWidget(self.label_22)

        self.azAccountKey_input = QLineEdit(self.verticalLayoutWidget_8)
        self.azAccountKey_input.setObjectName(u"azAccountKey_input")

        self.horizontalLayout_12.addWidget(self.azAccountKey_input)


        self.gridLayout.addLayout(self.horizontalLayout_12, 0, 0, 1, 1)

        self.horizontalLayout_13 = QHBoxLayout()
        self.horizontalLayout_13.setObjectName(u"horizontalLayout_13")
        self.label_23 = QLabel(self.verticalLayoutWidget_8)
        self.label_23.setObjectName(u"label_23")

        self.horizontalLayout_13.addWidget(self.label_23)

        self.azSasStorage_input = QLineEdit(self.verticalLayoutWidget_8)
        self.azSasStorage_input.setObjectName(u"azSasStorage_input")
        self.azSasStorage_input.setEnabled(False)

        self.horizontalLayout_13.addWidget(self.azSasStorage_input)


        self.gridLayout.addLayout(self.horizontalLayout_13, 1, 0, 1, 1)

        self.verticalLayout_12 = QVBoxLayout()
        self.verticalLayout_12.setObjectName(u"verticalLayout_12")
        self.horizontalLayout_14 = QHBoxLayout()
        self.horizontalLayout_14.setObjectName(u"horizontalLayout_14")
        self.label_24 = QLabel(self.verticalLayoutWidget_8)
        self.label_24.setObjectName(u"label_24")

        self.horizontalLayout_14.addWidget(self.label_24)

        self.azMsiAppid_input = QLineEdit(self.verticalLayoutWidget_8)
        self.azMsiAppid_input.setObjectName(u"azMsiAppid_input")
        self.azMsiAppid_input.setEnabled(False)

        self.horizontalLayout_14.addWidget(self.azMsiAppid_input)


        self.verticalLayout_12.addLayout(self.horizontalLayout_14)

        self.horizontalLayout_15 = QHBoxLayout()
        self.horizontalLayout_15.setObjectName(u"horizontalLayout_15")
        self.label_25 = QLabel(self.verticalLayoutWidget_8)
        self.label_25.setObjectName(u"label_25")

        self.horizontalLayout_15.addWidget(self.label_25)

        self.azMsiResourceid_input = QLineEdit(self.verticalLayoutWidget_8)
        self.azMsiResourceid_input.setObjectName(u"azMsiResourceid_input")
        self.azMsiResourceid_input.setEnabled(False)

        self.horizontalLayout_15.addWidget(self.azMsiResourceid_input)


        self.verticalLayout_12.addLayout(self.horizontalLayout_15)

        self.horizontalLayout_16 = QHBoxLayout()
        self.horizontalLayout_16.setObjectName(u"horizontalLayout_16")
        self.label_26 = QLabel(self.verticalLayoutWidget_8)
        self.label_26.setObjectName(u"label_26")

        self.horizontalLayout_16.addWidget(self.label_26)

        self.azMsiObjectid_input = QLineEdit(self.verticalLayoutWidget_8)
        self.azMsiObjectid_input.setObjectName(u"azMsiObjectid_input")
        self.azMsiObjectid_input.setEnabled(False)

        self.horizontalLayout_16.addWidget(self.azMsiObjectid_input)


        self.verticalLayout_12.addLayout(self.horizontalLayout_16)


        self.gridLayout.addLayout(self.verticalLayout_12, 1, 1, 1, 1)

        self.verticalLayout_13 = QVBoxLayout()
        self.verticalLayout_13.setObjectName(u"verticalLayout_13")
        self.horizontalLayout_17 = QHBoxLayout()
        self.horizontalLayout_17.setObjectName(u"horizontalLayout_17")
        self.label_27 = QLabel(self.verticalLayoutWidget_8)
        self.label_27.setObjectName(u"label_27")

        self.horizontalLayout_17.addWidget(self.label_27)

        self.azSpnTenantid_input = QLineEdit(self.verticalLayoutWidget_8)
        self.azSpnTenantid_input.setObjectName(u"azSpnTenantid_input")
        self.azSpnTenantid_input.setEnabled(False)

        self.horizontalLayout_17.addWidget(self.azSpnTenantid_input)


        self.verticalLayout_13.addLayout(self.horizontalLayout_17)

        self.horizontalLayout_18 = QHBoxLayout()
        self.horizontalLayout_18.setObjectName(u"horizontalLayout_18")
        self.label_28 = QLabel(self.verticalLayoutWidget_8)
        self.label_28.setObjectName(u"label_28")

        self.horizontalLayout_18.addWidget(self.label_28)

        self.azSpnclientid_input = QLineEdit(self.verticalLayoutWidget_8)
        self.azSpnclientid_input.setObjectName(u"azSpnclientid_input")
        self.azSpnclientid_input.setEnabled(False)

        self.horizontalLayout_18.addWidget(self.azSpnclientid_input)


        self.verticalLayout_13.addLayout(self.horizontalLayout_18)

        self.horizontalLayout_19 = QHBoxLayout()
        self.horizontalLayout_19.setObjectName(u"horizontalLayout_19")
        self.label_29 = QLabel(self.verticalLayoutWidget_8)
        self.label_29.setObjectName(u"label_29")

        self.horizontalLayout_19.addWidget(self.label_29)

        self.azSpnClientSecret_input = QLineEdit(self.verticalLayoutWidget_8)
        self.azSpnClientSecret_input.setObjectName(u"azSpnClientSecret_input")
        self.azSpnClientSecret_input.setEnabled(False)

        self.horizontalLayout_19.addWidget(self.azSpnClientSecret_input)


        self.verticalLayout_13.addLayout(self.horizontalLayout_19)


        self.gridLayout.addLayout(self.verticalLayout_13, 0, 1, 1, 1)


        self.verticalLayout_14.addLayout(self.gridLayout)

        self.AzureOptional_groupbox = QGroupBox(self.AzureBucket_groupbox)
        self.AzureOptional_groupbox.setObjectName(u"AzureOptional_groupbox")
        self.AzureOptional_groupbox.setGeometry(QRect(330, 30, 541, 431))
        self.AzureOptional_groupbox.setCheckable(True)
        self.AzureOptional_groupbox.setChecked(False)
        self.horizontalLayoutWidget_12 = QWidget(self.AzureOptional_groupbox)
        self.horizontalLayoutWidget_12.setObjectName(u"horizontalLayoutWidget_12")
        self.horizontalLayoutWidget_12.setGeometry(QRect(10, 20, 301, 401))
        self.horizontalLayout_20 = QHBoxLayout(self.horizontalLayoutWidget_12)
        self.horizontalLayout_20.setObjectName(u"horizontalLayout_20")
        self.horizontalLayout_20.setContentsMargins(0, 0, 0, 0)
        self.verticalLayout_15 = QVBoxLayout()
        self.verticalLayout_15.setObjectName(u"verticalLayout_15")
        self.label_30 = QLabel(self.horizontalLayoutWidget_12)
        self.label_30.setObjectName(u"label_30")

        self.verticalLayout_15.addWidget(self.label_30)

        self.label_31 = QLabel(self.horizontalLayoutWidget_12)
        self.label_31.setObjectName(u"label_31")

        self.verticalLayout_15.addWidget(self.label_31)

        self.label_32 = QLabel(self.horizontalLayoutWidget_12)
        self.label_32.setObjectName(u"label_32")

        self.verticalLayout_15.addWidget(self.label_32)

        self.label_33 = QLabel(self.horizontalLayoutWidget_12)
        self.label_33.setObjectName(u"label_33")

        self.verticalLayout_15.addWidget(self.label_33)

        self.label_34 = QLabel(self.horizontalLayoutWidget_12)
        self.label_34.setObjectName(u"label_34")

        self.verticalLayout_15.addWidget(self.label_34)

        self.label_35 = QLabel(self.horizontalLayoutWidget_12)
        self.label_35.setObjectName(u"label_35")

        self.verticalLayout_15.addWidget(self.label_35)

        self.label_36 = QLabel(self.horizontalLayoutWidget_12)
        self.label_36.setObjectName(u"label_36")

        self.verticalLayout_15.addWidget(self.label_36)

        self.label_37 = QLabel(self.horizontalLayoutWidget_12)
        self.label_37.setObjectName(u"label_37")

        self.verticalLayout_15.addWidget(self.label_37)

        self.label_38 = QLabel(self.horizontalLayoutWidget_12)
        self.label_38.setObjectName(u"label_38")

        self.verticalLayout_15.addWidget(self.label_38)

        self.label_39 = QLabel(self.horizontalLayoutWidget_12)
        self.label_39.setObjectName(u"label_39")

        self.verticalLayout_15.addWidget(self.label_39)

        self.label_40 = QLabel(self.horizontalLayoutWidget_12)
        self.label_40.setObjectName(u"label_40")

        self.verticalLayout_15.addWidget(self.label_40)

        self.label_41 = QLabel(self.horizontalLayoutWidget_12)
        self.label_41.setObjectName(u"label_41")

        self.verticalLayout_15.addWidget(self.label_41)

        self.label_42 = QLabel(self.horizontalLayoutWidget_12)
        self.label_42.setObjectName(u"label_42")

        self.verticalLayout_15.addWidget(self.label_42)


        self.horizontalLayout_20.addLayout(self.verticalLayout_15)

        self.verticalLayout_16 = QVBoxLayout()
        self.verticalLayout_16.setObjectName(u"verticalLayout_16")
        self.azAadEndpoint_input = QLineEdit(self.horizontalLayoutWidget_12)
        self.azAadEndpoint_input.setObjectName(u"azAadEndpoint_input")

        self.verticalLayout_16.addWidget(self.azAadEndpoint_input)

        self.azSubdirectory_input = QLineEdit(self.horizontalLayoutWidget_12)
        self.azSubdirectory_input.setObjectName(u"azSubdirectory_input")

        self.verticalLayout_16.addWidget(self.azSubdirectory_input)

        self.azblocksize_input = QLineEdit(self.horizontalLayoutWidget_12)
        self.azblocksize_input.setObjectName(u"azblocksize_input")

        self.verticalLayout_16.addWidget(self.azblocksize_input)

        self.azMaxconcurrency_input = QLineEdit(self.horizontalLayoutWidget_12)
        self.azMaxconcurrency_input.setObjectName(u"azMaxconcurrency_input")

        self.verticalLayout_16.addWidget(self.azMaxconcurrency_input)

        self.azblobtier_input = QComboBox(self.horizontalLayoutWidget_12)
        self.azblobtier_input.addItem("")
        self.azblobtier_input.addItem("")
        self.azblobtier_input.addItem("")
        self.azblobtier_input.addItem("")
        self.azblobtier_input.setObjectName(u"azblobtier_input")

        self.verticalLayout_16.addWidget(self.azblobtier_input)

        self.azBlocklistOnmount_input = QLineEdit(self.horizontalLayoutWidget_12)
        self.azBlocklistOnmount_input.setObjectName(u"azBlocklistOnmount_input")

        self.verticalLayout_16.addWidget(self.azBlocklistOnmount_input)

        self.azMaxretries_input = QLineEdit(self.horizontalLayoutWidget_12)
        self.azMaxretries_input.setObjectName(u"azMaxretries_input")

        self.verticalLayout_16.addWidget(self.azMaxretries_input)

        self.azMaxretrytimeout_input = QLineEdit(self.horizontalLayoutWidget_12)
        self.azMaxretrytimeout_input.setObjectName(u"azMaxretrytimeout_input")

        self.verticalLayout_16.addWidget(self.azMaxretrytimeout_input)

        self.azRetrybackoffs = QLineEdit(self.horizontalLayoutWidget_12)
        self.azRetrybackoffs.setObjectName(u"azRetrybackoffs")

        self.verticalLayout_16.addWidget(self.azRetrybackoffs)

        self.azMaxretrydelay_input = QLineEdit(self.horizontalLayoutWidget_12)
        self.azMaxretrydelay_input.setObjectName(u"azMaxretrydelay_input")

        self.verticalLayout_16.addWidget(self.azMaxretrydelay_input)

        self.azHttpproxy_input = QLineEdit(self.horizontalLayoutWidget_12)
        self.azHttpproxy_input.setObjectName(u"azHttpproxy_input")

        self.verticalLayout_16.addWidget(self.azHttpproxy_input)

        self.azHttpsproxy_input = QLineEdit(self.horizontalLayoutWidget_12)
        self.azHttpsproxy_input.setObjectName(u"azHttpsproxy_input")

        self.verticalLayout_16.addWidget(self.azHttpsproxy_input)

        self.azAuthresource_input = QLineEdit(self.horizontalLayoutWidget_12)
        self.azAuthresource_input.setObjectName(u"azAuthresource_input")

        self.verticalLayout_16.addWidget(self.azAuthresource_input)


        self.horizontalLayout_20.addLayout(self.verticalLayout_16)

        self.verticalLayoutWidget_11 = QWidget(self.AzureOptional_groupbox)
        self.verticalLayoutWidget_11.setObjectName(u"verticalLayoutWidget_11")
        self.verticalLayoutWidget_11.setGeometry(QRect(320, 20, 227, 170))
        self.verticalLayout_18 = QVBoxLayout(self.verticalLayoutWidget_11)
        self.verticalLayout_18.setObjectName(u"verticalLayout_18")
        self.verticalLayout_18.setContentsMargins(0, 0, 0, 0)
        self.azUseHttp = QCheckBox(self.verticalLayoutWidget_11)
        self.azUseHttp.setObjectName(u"azUseHttp")

        self.verticalLayout_18.addWidget(self.azUseHttp)

        self.azValidatemd5_checkbox = QCheckBox(self.verticalLayoutWidget_11)
        self.azValidatemd5_checkbox.setObjectName(u"azValidatemd5_checkbox")

        self.verticalLayout_18.addWidget(self.azValidatemd5_checkbox)

        self.azUpdatemd5_checkbox = QCheckBox(self.verticalLayoutWidget_11)
        self.azUpdatemd5_checkbox.setObjectName(u"azUpdatemd5_checkbox")

        self.verticalLayout_18.addWidget(self.azUpdatemd5_checkbox)

        self.azFailUnsupportedops_checkbox = QCheckBox(self.verticalLayoutWidget_11)
        self.azFailUnsupportedops_checkbox.setObjectName(u"azFailUnsupportedops_checkbox")

        self.verticalLayout_18.addWidget(self.azFailUnsupportedops_checkbox)

        self.azSdktrace_checkbox = QCheckBox(self.verticalLayoutWidget_11)
        self.azSdktrace_checkbox.setObjectName(u"azSdktrace_checkbox")

        self.verticalLayout_18.addWidget(self.azSdktrace_checkbox)

        self.azVirtualdirectory_checkbox = QCheckBox(self.verticalLayoutWidget_11)
        self.azVirtualdirectory_checkbox.setObjectName(u"azVirtualdirectory_checkbox")

        self.verticalLayout_18.addWidget(self.azVirtualdirectory_checkbox)


        self.retranslateUi(mountSettingsWidget)

        QMetaObject.connectSlotsByName(mountSettingsWidget)
    # setupUi

    def retranslateUi(self, mountSettingsWidget):
        mountSettingsWidget.setWindowTitle(QCoreApplication.translate("mountSettingsWidget", u"Form", None))
        self.libfuse_groupBox.setTitle(QCoreApplication.translate("mountSettingsWidget", u"Libfuse settings", None))
        self.label_3.setText(QCoreApplication.translate("mountSettingsWidget", u"Permissions", None))
        self.libfuse_permissions_select.setItemText(0, QCoreApplication.translate("mountSettingsWidget", u"0777", None))
        self.libfuse_permissions_select.setItemText(1, QCoreApplication.translate("mountSettingsWidget", u"0666", None))
        self.libfuse_permissions_select.setItemText(2, QCoreApplication.translate("mountSettingsWidget", u"0644", None))
        self.libfuse_permissions_select.setItemText(3, QCoreApplication.translate("mountSettingsWidget", u"0444", None))

#if QT_CONFIG(tooltip)
        self.libfuse_permissions_select.setToolTip(QCoreApplication.translate("mountSettingsWidget", u"<html><head/><body><p>Default permissions to be presented for block blobs</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_4.setText(QCoreApplication.translate("mountSettingsWidget", u"Attribute expiration (s)", None))
        self.label_5.setText(QCoreApplication.translate("mountSettingsWidget", u"Entry expiration (s)", None))
        self.label_6.setText(QCoreApplication.translate("mountSettingsWidget", u"Empty path expiration (s)", None))
        self.libfuse_attExp_input.setText(QCoreApplication.translate("mountSettingsWidget", u"120", None))
        self.libfuse_entExp_input.setText(QCoreApplication.translate("mountSettingsWidget", u"120", None))
        self.libfuse_pathExp_input.setText(QCoreApplication.translate("mountSettingsWidget", u"120", None))
        self.libfuse_disableWriteback_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Disable write-back cache", None))
        self.libfuse_ignoreAppend_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Ignore append/write only flags", None))
        self.streaming_groupBox.setTitle(QCoreApplication.translate("mountSettingsWidget", u"Streaming Settings", None))
        self.label_8.setText(QCoreApplication.translate("mountSettingsWidget", u"Block size (MB)", None))
        self.label_9.setText(QCoreApplication.translate("mountSettingsWidget", u"Max buffer (MB)", None))
        self.label_10.setText(QCoreApplication.translate("mountSettingsWidget", u"Buffer size (MB)", None))
        self.streaming_blksz_input.setText(QCoreApplication.translate("mountSettingsWidget", u"0", None))
        self.streaming_mxbuff_input.setText(QCoreApplication.translate("mountSettingsWidget", u"0", None))
        self.streaming_buffsz_input.setText(QCoreApplication.translate("mountSettingsWidget", u"0", None))
        self.label_11.setText(QCoreApplication.translate("mountSettingsWidget", u"File level caching", None))
        self.streaming_fileCachingLevel_select.setItemText(0, QCoreApplication.translate("mountSettingsWidget", u"Handle level", None))
        self.streaming_fileCachingLevel_select.setItemText(1, QCoreApplication.translate("mountSettingsWidget", u"File level", None))

        self.fileCache_groupBox.setTitle(QCoreApplication.translate("mountSettingsWidget", u"File Cache Settings", None))
        self.fileCacheAdvancedSettings_groupBox.setTitle(QCoreApplication.translate("mountSettingsWidget", u"Advanced", None))
        self.label_14.setText(QCoreApplication.translate("mountSettingsWidget", u"Eviction Policy", None))
        self.label_15.setText(QCoreApplication.translate("mountSettingsWidget", u"Eviction timeout (s)", None))
        self.label_16.setText(QCoreApplication.translate("mountSettingsWidget", u"Max Eviction", None))
        self.label_17.setText(QCoreApplication.translate("mountSettingsWidget", u"Max cache size (MB)", None))
        self.label_18.setText(QCoreApplication.translate("mountSettingsWidget", u"Eviction max threshold (%)", None))
        self.label_19.setText(QCoreApplication.translate("mountSettingsWidget", u"Eviction min threshold (%)", None))
        self.fileCache_evictionPolicy_select.setItemText(0, QCoreApplication.translate("mountSettingsWidget", u"lru - least recently used", None))
        self.fileCache_evictionPolicy_select.setItemText(1, QCoreApplication.translate("mountSettingsWidget", u"lfu - least frequently used", None))

        self.fileCache_timeout_input.setText(QCoreApplication.translate("mountSettingsWidget", u"120", None))
        self.fileCache_maxEviction_input.setText(QCoreApplication.translate("mountSettingsWidget", u"5000", None))
#if QT_CONFIG(tooltip)
        self.fileCache_maxCacheSize_input.setToolTip(QCoreApplication.translate("mountSettingsWidget", u"<html><head/><body><p>Enter cache size in MB - 0 indicates unlimited</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.fileCache_maxCacheSize_input.setText(QCoreApplication.translate("mountSettingsWidget", u"0", None))
        self.fileCache_evictMaxThresh_input.setText(QCoreApplication.translate("mountSettingsWidget", u"80", None))
        self.fileCache_evictMinThresh_input.setText(QCoreApplication.translate("mountSettingsWidget", u"60", None))
        self.fileCache_allowNonEmptyTmp_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Allow non-empty temp", None))
        self.fileCache_policyLogs_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Policy logs", None))
        self.fileCache_createEmptyFile_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Create empty file", None))
        self.fileCache_cleanupStart_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Cleanup on start", None))
        self.fileCache_offloadIO_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Offload io", None))
        self.label_13.setText(QCoreApplication.translate("mountSettingsWidget", u"File cache path", None))
#if QT_CONFIG(tooltip)
        self.commonConfig_multiuser_checkbox.setToolTip(QCoreApplication.translate("mountSettingsWidget", u"<html><head/><body><p>Select to allow multiple users to access the container</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.commonConfig_multiuser_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Multiple Users", None))
        self.commonConfig_nonEmptyDir_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Non-empty directory mount", None))
        self.commonConfig_readOnly_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Read-only mount", None))
        self.daemonForeground_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Run in foreground", None))
        self.label.setText(QCoreApplication.translate("mountSettingsWidget", u"ALPHA DESIGN", None))
        self.pushButton.setText(QCoreApplication.translate("mountSettingsWidget", u"Reset Default Settings", None))
        self.AzureBucket_groupbox.setTitle(QCoreApplication.translate("mountSettingsWidget", u"Azure Bucket Settings", None))
        self.label_2.setText(QCoreApplication.translate("mountSettingsWidget", u"Type", None))
        self.AzTypesettings_select.setItemText(0, QCoreApplication.translate("mountSettingsWidget", u"block", None))
        self.AzTypesettings_select.setItemText(1, QCoreApplication.translate("mountSettingsWidget", u"adls", None))

        self.label_7.setText(QCoreApplication.translate("mountSettingsWidget", u"Account name", None))
        self.label_12.setText(QCoreApplication.translate("mountSettingsWidget", u"Container", None))
        self.label_20.setText(QCoreApplication.translate("mountSettingsWidget", u"Endpoint", None))
        self.label_21.setText(QCoreApplication.translate("mountSettingsWidget", u"Mode", None))
        self.azModesettings_select.setItemText(0, QCoreApplication.translate("mountSettingsWidget", u"key", None))
        self.azModesettings_select.setItemText(1, QCoreApplication.translate("mountSettingsWidget", u"sas", None))
        self.azModesettings_select.setItemText(2, QCoreApplication.translate("mountSettingsWidget", u"spn", None))
        self.azModesettings_select.setItemText(3, QCoreApplication.translate("mountSettingsWidget", u"msi", None))

        self.label_22.setText(QCoreApplication.translate("mountSettingsWidget", u"Account key", None))
        self.label_23.setText(QCoreApplication.translate("mountSettingsWidget", u"Sas storage", None))
        self.label_24.setText(QCoreApplication.translate("mountSettingsWidget", u"App ID", None))
        self.label_25.setText(QCoreApplication.translate("mountSettingsWidget", u"Resource ID", None))
        self.azMsiResourceid_input.setText("")
        self.label_26.setText(QCoreApplication.translate("mountSettingsWidget", u"Object ID", None))
        self.azMsiObjectid_input.setText("")
        self.label_27.setText(QCoreApplication.translate("mountSettingsWidget", u"Tenant ID", None))
        self.label_28.setText(QCoreApplication.translate("mountSettingsWidget", u"Client ID", None))
        self.azSpnclientid_input.setText("")
        self.label_29.setText(QCoreApplication.translate("mountSettingsWidget", u"Client Secret", None))
        self.azSpnClientSecret_input.setText("")
        self.AzureOptional_groupbox.setTitle(QCoreApplication.translate("mountSettingsWidget", u"Optional", None))
        self.label_30.setText(QCoreApplication.translate("mountSettingsWidget", u"Aad endpoint", None))
        self.label_31.setText(QCoreApplication.translate("mountSettingsWidget", u"Subdirectory", None))
        self.label_32.setText(QCoreApplication.translate("mountSettingsWidget", u"Block-size (MB)", None))
        self.label_33.setText(QCoreApplication.translate("mountSettingsWidget", u"Max concurrency", None))
        self.label_34.setText(QCoreApplication.translate("mountSettingsWidget", u"Blob tier", None))
        self.label_35.setText(QCoreApplication.translate("mountSettingsWidget", u"Block on mount (s)", None))
        self.label_36.setText(QCoreApplication.translate("mountSettingsWidget", u"Max retries", None))
        self.label_37.setText(QCoreApplication.translate("mountSettingsWidget", u"Max retry timeout (s)", None))
        self.label_38.setText(QCoreApplication.translate("mountSettingsWidget", u"Retry backoff (s)", None))
        self.label_39.setText(QCoreApplication.translate("mountSettingsWidget", u"Max Retry Delay (s)", None))
        self.label_40.setText(QCoreApplication.translate("mountSettingsWidget", u"http proxy", None))
        self.label_41.setText(QCoreApplication.translate("mountSettingsWidget", u"Https proxy", None))
        self.label_42.setText(QCoreApplication.translate("mountSettingsWidget", u"Auth resource", None))
        self.azblocksize_input.setText(QCoreApplication.translate("mountSettingsWidget", u"16", None))
        self.azMaxconcurrency_input.setText(QCoreApplication.translate("mountSettingsWidget", u"32", None))
        self.azblobtier_input.setItemText(0, QCoreApplication.translate("mountSettingsWidget", u"none", None))
        self.azblobtier_input.setItemText(1, QCoreApplication.translate("mountSettingsWidget", u"hot", None))
        self.azblobtier_input.setItemText(2, QCoreApplication.translate("mountSettingsWidget", u"cool", None))
        self.azblobtier_input.setItemText(3, QCoreApplication.translate("mountSettingsWidget", u"archive", None))

        self.azBlocklistOnmount_input.setText(QCoreApplication.translate("mountSettingsWidget", u"0", None))
        self.azMaxretries_input.setText(QCoreApplication.translate("mountSettingsWidget", u"5", None))
        self.azMaxretrytimeout_input.setText(QCoreApplication.translate("mountSettingsWidget", u"900", None))
        self.azRetrybackoffs.setText(QCoreApplication.translate("mountSettingsWidget", u"4", None))
        self.azMaxretrydelay_input.setText(QCoreApplication.translate("mountSettingsWidget", u"60", None))
#if QT_CONFIG(tooltip)
        self.azUseHttp.setToolTip(QCoreApplication.translate("mountSettingsWidget", u"<html><head/><body><p>Use http instead of https</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.azUseHttp.setText(QCoreApplication.translate("mountSettingsWidget", u"Use http", None))
        self.azValidatemd5_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Validate md5 (file cache only)", None))
        self.azUpdatemd5_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Update md5 (file cache only)", None))
        self.azFailUnsupportedops_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Fail Unsupported Ops", None))
        self.azSdktrace_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Sdk trace", None))
        self.azVirtualdirectory_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Virtual directory", None))
    # retranslateUi

