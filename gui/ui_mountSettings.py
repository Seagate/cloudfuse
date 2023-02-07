# -*- coding: utf-8 -*-

################################################################################
## Form generated from reading UI file 'MountSettings.ui'
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
from PySide6.QtWidgets import (QApplication, QCheckBox, QComboBox, QGroupBox,
    QHBoxLayout, QLabel, QLineEdit, QSizePolicy,
    QVBoxLayout, QWidget)

class Ui_mountSettingsWidget(object):
    def setupUi(self, mountSettingsWidget):
        if not mountSettingsWidget.objectName():
            mountSettingsWidget.setObjectName(u"mountSettingsWidget")
        mountSettingsWidget.resize(1137, 733)
        self.previousSettings_checkbox = QCheckBox(mountSettingsWidget)
        self.previousSettings_checkbox.setObjectName(u"previousSettings_checkbox")
        self.previousSettings_checkbox.setGeometry(QRect(960, 690, 171, 23))
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

        self.resetDefaultSettings_checkbox = QCheckBox(mountSettingsWidget)
        self.resetDefaultSettings_checkbox.setObjectName(u"resetDefaultSettings_checkbox")
        self.resetDefaultSettings_checkbox.setGeometry(QRect(760, 690, 191, 23))
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

        self.verticalLayout_5.addWidget(self.streaming_blksz_input)

        self.streaming_mxbuff_input = QLineEdit(self.verticalLayoutWidget_6)
        self.streaming_mxbuff_input.setObjectName(u"streaming_mxbuff_input")
        self.streaming_mxbuff_input.setMaximumSize(QSize(75, 16777215))

        self.verticalLayout_5.addWidget(self.streaming_mxbuff_input)

        self.streaming_buffsz_input = QLineEdit(self.verticalLayoutWidget_6)
        self.streaming_buffsz_input.setObjectName(u"streaming_buffsz_input")
        self.streaming_buffsz_input.setMaximumSize(QSize(75, 16777215))

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

        self.verticalLayout_7.addWidget(self.fileCache_timeout_input)

        self.fileCache_maxEviction_input = QLineEdit(self.verticalLayoutWidget_2)
        self.fileCache_maxEviction_input.setObjectName(u"fileCache_maxEviction_input")
        self.fileCache_maxEviction_input.setMaximumSize(QSize(75, 16777215))

        self.verticalLayout_7.addWidget(self.fileCache_maxEviction_input)

        self.fileCache_maxCacheSize_input = QLineEdit(self.verticalLayoutWidget_2)
        self.fileCache_maxCacheSize_input.setObjectName(u"fileCache_maxCacheSize_input")
        self.fileCache_maxCacheSize_input.setMaximumSize(QSize(75, 16777215))

        self.verticalLayout_7.addWidget(self.fileCache_maxCacheSize_input)

        self.fileCache_evictMaxThresh_input = QLineEdit(self.verticalLayoutWidget_2)
        self.fileCache_evictMaxThresh_input.setObjectName(u"fileCache_evictMaxThresh_input")
        self.fileCache_evictMaxThresh_input.setMaximumSize(QSize(30, 16777215))

        self.verticalLayout_7.addWidget(self.fileCache_evictMaxThresh_input)

        self.fileCache_evictMinThresh_input = QLineEdit(self.verticalLayoutWidget_2)
        self.fileCache_evictMinThresh_input.setObjectName(u"fileCache_evictMinThresh_input")
        self.fileCache_evictMinThresh_input.setMaximumSize(QSize(30, 16777215))

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

        self.retranslateUi(mountSettingsWidget)

        QMetaObject.connectSlotsByName(mountSettingsWidget)
    # setupUi

    def retranslateUi(self, mountSettingsWidget):
        mountSettingsWidget.setWindowTitle(QCoreApplication.translate("mountSettingsWidget", u"Form", None))
#if QT_CONFIG(tooltip)
        self.previousSettings_checkbox.setToolTip(QCoreApplication.translate("mountSettingsWidget", u"<html><head/><body><p>Use previously saved settings</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.previousSettings_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Use previous settings", None))
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
        self.resetDefaultSettings_checkbox.setText(QCoreApplication.translate("mountSettingsWidget", u"Reset settings to default", None))
        self.streaming_groupBox.setTitle(QCoreApplication.translate("mountSettingsWidget", u"Streaming Settings", None))
        self.label_8.setText(QCoreApplication.translate("mountSettingsWidget", u"Block size (MB)", None))
        self.label_9.setText(QCoreApplication.translate("mountSettingsWidget", u"Max buffer (MB)", None))
        self.label_10.setText(QCoreApplication.translate("mountSettingsWidget", u"Buffer size (MB)", None))
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
    # retranslateUi

