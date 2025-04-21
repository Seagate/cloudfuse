# -*- coding: utf-8 -*-

################################################################################
## Form generated from reading UI file 's3_config_advanced.ui'
##
## Created by: Qt User Interface Compiler version 6.8.2
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
    QPushButton, QSizePolicy, QSpinBox, QVBoxLayout,
    QWidget)

class Ui_Form(object):
    def setupUi(self, Form):
        if not Form.objectName():
            Form.setObjectName(u"Form")
        Form.resize(640, 556)
        self.gridLayout = QGridLayout(Form)
        self.gridLayout.setObjectName(u"gridLayout")
        self.button_resetDefaultSettings = QPushButton(Form)
        self.button_resetDefaultSettings.setObjectName(u"button_resetDefaultSettings")
        self.button_resetDefaultSettings.setEnabled(True)
        self.button_resetDefaultSettings.setMaximumSize(QSize(165, 16777215))
        self.button_resetDefaultSettings.setLayoutDirection(Qt.LeftToRight)

        self.gridLayout.addWidget(self.button_resetDefaultSettings, 7, 0, 1, 1)

        self.button_okay = QPushButton(Form)
        self.button_okay.setObjectName(u"button_okay")
        self.button_okay.setMaximumSize(QSize(100, 16777215))
        self.button_okay.setLayoutDirection(Qt.RightToLeft)

        self.gridLayout.addWidget(self.button_okay, 7, 1, 1, 1)

        self.groupBox = QGroupBox(Form)
        self.groupBox.setObjectName(u"groupBox")
        self.groupBox.setMinimumSize(QSize(275, 80))
        self.groupBox.setFlat(False)
        self.horizontalLayoutWidget = QWidget(self.groupBox)
        self.horizontalLayoutWidget.setObjectName(u"horizontalLayoutWidget")
        self.horizontalLayoutWidget.setGeometry(QRect(9, 30, 581, 41))
        self.horizontalLayout = QHBoxLayout(self.horizontalLayoutWidget)
        self.horizontalLayout.setObjectName(u"horizontalLayout")
        self.horizontalLayout.setContentsMargins(0, 0, 0, 0)
        self.label = QLabel(self.horizontalLayoutWidget)
        self.label.setObjectName(u"label")

        self.horizontalLayout.addWidget(self.label)

        self.lineEdit_subdirectory = QLineEdit(self.horizontalLayoutWidget)
        self.lineEdit_subdirectory.setObjectName(u"lineEdit_subdirectory")

        self.horizontalLayout.addWidget(self.lineEdit_subdirectory)


        self.gridLayout.addWidget(self.groupBox, 2, 0, 1, 2)

        self.groupbox_fileCache = QGroupBox(Form)
        self.groupbox_fileCache.setObjectName(u"groupbox_fileCache")
        self.groupbox_fileCache.setEnabled(True)
        sizePolicy = QSizePolicy(QSizePolicy.Policy.Preferred, QSizePolicy.Policy.Preferred)
        sizePolicy.setHorizontalStretch(0)
        sizePolicy.setVerticalStretch(0)
        sizePolicy.setHeightForWidth(self.groupbox_fileCache.sizePolicy().hasHeightForWidth())
        self.groupbox_fileCache.setSizePolicy(sizePolicy)
        font = QFont()
        font.setKerning(True)
        self.groupbox_fileCache.setFont(font)
        self.groupbox_fileCache.setAcceptDrops(False)
        self.groupbox_fileCache.setAutoFillBackground(False)
        self.gridLayout_2 = QGridLayout(self.groupbox_fileCache)
        self.gridLayout_2.setObjectName(u"gridLayout_2")
        self.verticalLayout_9 = QVBoxLayout()
        self.verticalLayout_9.setObjectName(u"verticalLayout_9")
        self.checkBox_fileCache_allowNonEmptyTmp = QCheckBox(self.groupbox_fileCache)
        self.checkBox_fileCache_allowNonEmptyTmp.setObjectName(u"checkBox_fileCache_allowNonEmptyTmp")

        self.verticalLayout_9.addWidget(self.checkBox_fileCache_allowNonEmptyTmp)

        self.checkBox_fileCache_policyLogs = QCheckBox(self.groupbox_fileCache)
        self.checkBox_fileCache_policyLogs.setObjectName(u"checkBox_fileCache_policyLogs")

        self.verticalLayout_9.addWidget(self.checkBox_fileCache_policyLogs)

        self.checkBox_fileCache_createEmptyFile = QCheckBox(self.groupbox_fileCache)
        self.checkBox_fileCache_createEmptyFile.setObjectName(u"checkBox_fileCache_createEmptyFile")

        self.verticalLayout_9.addWidget(self.checkBox_fileCache_createEmptyFile)

        self.checkBox_fileCache_cleanupStart = QCheckBox(self.groupbox_fileCache)
        self.checkBox_fileCache_cleanupStart.setObjectName(u"checkBox_fileCache_cleanupStart")

        self.verticalLayout_9.addWidget(self.checkBox_fileCache_cleanupStart)

        self.checkBox_fileCache_offloadIO = QCheckBox(self.groupbox_fileCache)
        self.checkBox_fileCache_offloadIO.setObjectName(u"checkBox_fileCache_offloadIO")

        self.verticalLayout_9.addWidget(self.checkBox_fileCache_offloadIO)

        self.checkBox_fileCache_syncToFlush = QCheckBox(self.groupbox_fileCache)
        self.checkBox_fileCache_syncToFlush.setObjectName(u"checkBox_fileCache_syncToFlush")

        self.verticalLayout_9.addWidget(self.checkBox_fileCache_syncToFlush)


        self.gridLayout_2.addLayout(self.verticalLayout_9, 0, 0, 1, 1)

        self.horizontalLayout_7 = QHBoxLayout()
        self.horizontalLayout_7.setObjectName(u"horizontalLayout_7")
        self.verticalLayout_8 = QVBoxLayout()
        self.verticalLayout_8.setObjectName(u"verticalLayout_8")
        self.label_15 = QLabel(self.groupbox_fileCache)
        self.label_15.setObjectName(u"label_15")

        self.verticalLayout_8.addWidget(self.label_15)

        self.label_16 = QLabel(self.groupbox_fileCache)
        self.label_16.setObjectName(u"label_16")

        self.verticalLayout_8.addWidget(self.label_16)

        self.label_17 = QLabel(self.groupbox_fileCache)
        self.label_17.setObjectName(u"label_17")

        self.verticalLayout_8.addWidget(self.label_17)

        self.label_18 = QLabel(self.groupbox_fileCache)
        self.label_18.setObjectName(u"label_18")

        self.verticalLayout_8.addWidget(self.label_18)

        self.label_19 = QLabel(self.groupbox_fileCache)
        self.label_19.setObjectName(u"label_19")

        self.verticalLayout_8.addWidget(self.label_19)

        self.label_3 = QLabel(self.groupbox_fileCache)
        self.label_3.setObjectName(u"label_3")

        self.verticalLayout_8.addWidget(self.label_3)


        self.horizontalLayout_7.addLayout(self.verticalLayout_8)

        self.verticalLayout_7 = QVBoxLayout()
        self.verticalLayout_7.setObjectName(u"verticalLayout_7")
        self.dropDown_fileCache_evictionPolicy = QComboBox(self.groupbox_fileCache)
        self.dropDown_fileCache_evictionPolicy.addItem("")
        self.dropDown_fileCache_evictionPolicy.addItem("")
        self.dropDown_fileCache_evictionPolicy.setObjectName(u"dropDown_fileCache_evictionPolicy")

        self.verticalLayout_7.addWidget(self.dropDown_fileCache_evictionPolicy)

        self.spinBox_fileCache_evictionTimeout = QSpinBox(self.groupbox_fileCache)
        self.spinBox_fileCache_evictionTimeout.setObjectName(u"spinBox_fileCache_evictionTimeout")
        self.spinBox_fileCache_evictionTimeout.setMaximum(2147483647)
        self.spinBox_fileCache_evictionTimeout.setSingleStep(10)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_evictionTimeout)

        self.spinBox_fileCache_maxEviction = QSpinBox(self.groupbox_fileCache)
        self.spinBox_fileCache_maxEviction.setObjectName(u"spinBox_fileCache_maxEviction")
        self.spinBox_fileCache_maxEviction.setMaximum(2147483647)
        self.spinBox_fileCache_maxEviction.setSingleStep(20)
        self.spinBox_fileCache_maxEviction.setValue(0)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_maxEviction)

        self.spinBox_fileCache_maxCacheSize = QSpinBox(self.groupbox_fileCache)
        self.spinBox_fileCache_maxCacheSize.setObjectName(u"spinBox_fileCache_maxCacheSize")
        self.spinBox_fileCache_maxCacheSize.setMaximum(2147483647)
        self.spinBox_fileCache_maxCacheSize.setSingleStep(100)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_maxCacheSize)

        self.spinBox_fileCache_evictMaxThresh = QSpinBox(self.groupbox_fileCache)
        self.spinBox_fileCache_evictMaxThresh.setObjectName(u"spinBox_fileCache_evictMaxThresh")
        self.spinBox_fileCache_evictMaxThresh.setMaximum(2147483647)
        self.spinBox_fileCache_evictMaxThresh.setSingleStep(5)
        self.spinBox_fileCache_evictMaxThresh.setValue(80)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_evictMaxThresh)

        self.spinBox_fileCache_evictMinThresh = QSpinBox(self.groupbox_fileCache)
        self.spinBox_fileCache_evictMinThresh.setObjectName(u"spinBox_fileCache_evictMinThresh")
        self.spinBox_fileCache_evictMinThresh.setMaximum(2147483647)
        self.spinBox_fileCache_evictMinThresh.setSingleStep(5)
        self.spinBox_fileCache_evictMinThresh.setValue(60)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_evictMinThresh)

        self.spinBox_fileCache_refreshSec = QSpinBox(self.groupbox_fileCache)
        self.spinBox_fileCache_refreshSec.setObjectName(u"spinBox_fileCache_refreshSec")
        self.spinBox_fileCache_refreshSec.setMaximum(2147483647)
        self.spinBox_fileCache_refreshSec.setSingleStep(20)
        self.spinBox_fileCache_refreshSec.setValue(60)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_refreshSec)


        self.horizontalLayout_7.addLayout(self.verticalLayout_7)


        self.gridLayout_2.addLayout(self.horizontalLayout_7, 0, 1, 1, 1)


        self.gridLayout.addWidget(self.groupbox_fileCache, 1, 0, 1, 2)

        self.groupBox_2 = QGroupBox(Form)
        self.groupBox_2.setObjectName(u"groupBox_2")
        self.groupBox_2.setMinimumSize(QSize(600, 150))
        self.verticalLayoutWidget = QWidget(self.groupBox_2)
        self.verticalLayoutWidget.setObjectName(u"verticalLayoutWidget")
        self.verticalLayoutWidget.setGeometry(QRect(10, 20, 581, 121))
        self.verticalLayout = QVBoxLayout(self.verticalLayoutWidget)
        self.verticalLayout.setObjectName(u"verticalLayout")
        self.verticalLayout.setContentsMargins(0, 0, 0, 0)
        self.horizontalLayout_2 = QHBoxLayout()
        self.horizontalLayout_2.setObjectName(u"horizontalLayout_2")
        self.label_2 = QLabel(self.verticalLayoutWidget)
        self.label_2.setObjectName(u"label_2")

        self.horizontalLayout_2.addWidget(self.label_2)

        self.spinBox_libfuse_maxFuseThreads = QSpinBox(self.verticalLayoutWidget)
        self.spinBox_libfuse_maxFuseThreads.setObjectName(u"spinBox_libfuse_maxFuseThreads")
        self.spinBox_libfuse_maxFuseThreads.setMaximum(2147483647)
        self.spinBox_libfuse_maxFuseThreads.setSingleStep(20)
        self.spinBox_libfuse_maxFuseThreads.setValue(128)

        self.horizontalLayout_2.addWidget(self.spinBox_libfuse_maxFuseThreads)


        self.verticalLayout.addLayout(self.horizontalLayout_2)

        self.horizontalLayout_3 = QHBoxLayout()
        self.horizontalLayout_3.setObjectName(u"horizontalLayout_3")
        self.checkBox_libfuse_networkshare = QCheckBox(self.verticalLayoutWidget)
        self.checkBox_libfuse_networkshare.setObjectName(u"checkBox_libfuse_networkshare")

        self.horizontalLayout_3.addWidget(self.checkBox_libfuse_networkshare)

        self.checkBox_libfuse_disableWriteback = QCheckBox(self.verticalLayoutWidget)
        self.checkBox_libfuse_disableWriteback.setObjectName(u"checkBox_libfuse_disableWriteback")

        self.horizontalLayout_3.addWidget(self.checkBox_libfuse_disableWriteback)


        self.verticalLayout.addLayout(self.horizontalLayout_3)


        self.gridLayout.addWidget(self.groupBox_2, 3, 0, 1, 2)


        self.retranslateUi(Form)

        self.button_resetDefaultSettings.setDefault(False)


        QMetaObject.connectSlotsByName(Form)
    # setupUi

    def retranslateUi(self, Form):
        Form.setWindowTitle(QCoreApplication.translate("Form", u"Form", None))
#if QT_CONFIG(tooltip)
        self.button_resetDefaultSettings.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Reset to previous settings - does not take effect until changes are saved</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.button_resetDefaultSettings.setText(QCoreApplication.translate("Form", u"Reset Changes", None))
        self.button_okay.setText(QCoreApplication.translate("Form", u"Save", None))
        self.groupBox.setTitle(QCoreApplication.translate("Form", u"S3", None))
#if QT_CONFIG(tooltip)
        self.label.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The name of the subdirectory to be mounted instead of the whole bucket</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label.setText(QCoreApplication.translate("Form", u"Sub-directory", None))
        self.groupbox_fileCache.setTitle(QCoreApplication.translate("Form", u"File Caching", None))
#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_allowNonEmptyTmp.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Keep local file cache on unmount and remount (allow-non-empty-temp)</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_allowNonEmptyTmp.setText(QCoreApplication.translate("Form", u"Persist File Cache", None))
#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_policyLogs.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Generate eviction policy logs showing which files will expire soon</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_policyLogs.setText(QCoreApplication.translate("Form", u"Policy Logs", None))
#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_createEmptyFile.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Create an empty file on the container when create call is received from the kernel</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_createEmptyFile.setText(QCoreApplication.translate("Form", u"Create Empty File", None))
#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_cleanupStart.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Clean the temp directory on startup if it is not empty already</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_cleanupStart.setText(QCoreApplication.translate("Form", u"Cleanup on Start", None))
#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_offloadIO.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>By default, the libfuse component will service reads/writes to files for better performance. Check the box to make the file-cache component service read/write calls as well.</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_offloadIO.setText(QCoreApplication.translate("Form", u"Offload IO", None))
#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_syncToFlush.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Sync call to a file will force upload of the contents to the storage account</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_syncToFlush.setText(QCoreApplication.translate("Form", u"Sync to Flush", None))
#if QT_CONFIG(tooltip)
        self.label_15.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The amount of time (seconds) set to for eviction in the cache </p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_15.setText(QCoreApplication.translate("Form", u"Eviction Timeout (s)", None))
#if QT_CONFIG(tooltip)
        self.label_16.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The number of files that can be evicted at once - default 5,000</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_16.setText(QCoreApplication.translate("Form", u"Max Eviction", None))
#if QT_CONFIG(tooltip)
        self.label_17.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The maximum cache size allowed in MB - set to zero for unlimited</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_17.setText(QCoreApplication.translate("Form", u"Max Cache Size (MB)", None))
#if QT_CONFIG(tooltip)
        self.label_18.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The percentage of disk space consumed when the eviction is triggers. This parameter overrides the eviction timeout parameter and cached files will be removed even if they have not expired. </p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_18.setText(QCoreApplication.translate("Form", u"Eviction Max Threshold (%)", None))
#if QT_CONFIG(tooltip)
        self.label_19.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The percentage of disk space consumed which triggers the eviction to STOP evicting files when previously triggered by the high-threshold setting.</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_19.setText(QCoreApplication.translate("Form", u"Eviction Min Threshold (%)", None))
#if QT_CONFIG(tooltip)
        self.label_3.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The time (in seconds) between checks to see if a file in the local cache is up to date with the container's latest copy. </p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_3.setText(QCoreApplication.translate("Form", u"Cache Update File Timeout(s)", None))
        self.dropDown_fileCache_evictionPolicy.setItemText(0, QCoreApplication.translate("Form", u"lru - least recently used", None))
        self.dropDown_fileCache_evictionPolicy.setItemText(1, QCoreApplication.translate("Form", u"lfu - least frequently used", None))

        self.groupBox_2.setTitle(QCoreApplication.translate("Form", u"LibFuse", None))
#if QT_CONFIG(tooltip)
        self.label_2.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The number of threads allowed at the libfuse layer for highly parallel operations</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_2.setText(QCoreApplication.translate("Form", u"Maximum Fuse Threads", None))
#if QT_CONFIG(tooltip)
        self.checkBox_libfuse_networkshare.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Runs as a network share - may improve performance when latency to cloud is high. </p><p><span style=\" font-weight:600;\">ONLY supported on Windows.</span></p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_libfuse_networkshare.setText(QCoreApplication.translate("Form", u"Enable Network-share", None))
#if QT_CONFIG(tooltip)
        self.checkBox_libfuse_disableWriteback.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Dis-allow libfuse to buffer write requests if one must stricty open files in write only or append mode. Alternatively, just set ignore open flags in general settings</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_libfuse_disableWriteback.setText(QCoreApplication.translate("Form", u"Disable Write-back Cache", None))
    # retranslateUi

