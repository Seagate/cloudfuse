# -*- coding: utf-8 -*-

################################################################################
## Form generated from reading UI file 'azure_config_advanced.ui'
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
        Form.resize(792, 706)
        self.gridLayout = QGridLayout(Form)
        self.gridLayout.setObjectName(u"gridLayout")
        self.groupBox_libfuse = QGroupBox(Form)
        self.groupBox_libfuse.setObjectName(u"groupBox_libfuse")
        self.groupBox_libfuse.setMinimumSize(QSize(450, 125))
        self.verticalLayoutWidget = QWidget(self.groupBox_libfuse)
        self.verticalLayoutWidget.setObjectName(u"verticalLayoutWidget")
        self.verticalLayoutWidget.setGeometry(QRect(10, 30, 421, 98))
        self.verticalLayout = QVBoxLayout(self.verticalLayoutWidget)
        self.verticalLayout.setObjectName(u"verticalLayout")
        self.verticalLayout.setContentsMargins(0, 0, 0, 0)
        self.horizontalLayout = QHBoxLayout()
        self.horizontalLayout.setObjectName(u"horizontalLayout")
        self.label = QLabel(self.verticalLayoutWidget)
        self.label.setObjectName(u"label")

        self.horizontalLayout.addWidget(self.label)

        self.spinBox_libfuse_maxFuseThreads = QSpinBox(self.verticalLayoutWidget)
        self.spinBox_libfuse_maxFuseThreads.setObjectName(u"spinBox_libfuse_maxFuseThreads")
        self.spinBox_libfuse_maxFuseThreads.setMinimumSize(QSize(120, 0))
        self.spinBox_libfuse_maxFuseThreads.setMaximumSize(QSize(16777215, 16777215))
        self.spinBox_libfuse_maxFuseThreads.setFocusPolicy(Qt.NoFocus)
        self.spinBox_libfuse_maxFuseThreads.setMaximum(2147483647)
        self.spinBox_libfuse_maxFuseThreads.setSingleStep(20)
        self.spinBox_libfuse_maxFuseThreads.setValue(128)

        self.horizontalLayout.addWidget(self.spinBox_libfuse_maxFuseThreads)


        self.verticalLayout.addLayout(self.horizontalLayout)

        self.horizontalLayout_3 = QHBoxLayout()
        self.horizontalLayout_3.setObjectName(u"horizontalLayout_3")
        self.checkBox_libfuse_networkshare = QCheckBox(self.verticalLayoutWidget)
        self.checkBox_libfuse_networkshare.setObjectName(u"checkBox_libfuse_networkshare")

        self.horizontalLayout_3.addWidget(self.checkBox_libfuse_networkshare)

        self.checkBox_libfuse_disableWriteback = QCheckBox(self.verticalLayoutWidget)
        self.checkBox_libfuse_disableWriteback.setObjectName(u"checkBox_libfuse_disableWriteback")

        self.horizontalLayout_3.addWidget(self.checkBox_libfuse_disableWriteback)


        self.verticalLayout.addLayout(self.horizontalLayout_3)


        self.gridLayout.addWidget(self.groupBox_libfuse, 1, 0, 1, 1)

        self.groupbox_fileCache = QGroupBox(Form)
        self.groupbox_fileCache.setObjectName(u"groupbox_fileCache")
        self.gridLayout_2 = QGridLayout(self.groupbox_fileCache)
        self.gridLayout_2.setObjectName(u"gridLayout_2")
        self.verticalLayout_9 = QVBoxLayout()
        self.verticalLayout_9.setObjectName(u"verticalLayout_9")
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

        self.label_2 = QLabel(self.groupbox_fileCache)
        self.label_2.setObjectName(u"label_2")

        self.verticalLayout_8.addWidget(self.label_2)


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
        self.spinBox_fileCache_evictionTimeout.setValue(120)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_evictionTimeout)

        self.spinBox_fileCache_maxEviction = QSpinBox(self.groupbox_fileCache)
        self.spinBox_fileCache_maxEviction.setObjectName(u"spinBox_fileCache_maxEviction")
        self.spinBox_fileCache_maxEviction.setMaximum(2147483647)
        self.spinBox_fileCache_maxEviction.setValue(5000)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_maxEviction)

        self.spinBox_fileCache_maxCacheSize = QSpinBox(self.groupbox_fileCache)
        self.spinBox_fileCache_maxCacheSize.setObjectName(u"spinBox_fileCache_maxCacheSize")
        self.spinBox_fileCache_maxCacheSize.setMaximum(2147483647)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_maxCacheSize)

        self.spinBox_fileCache_evictMaxThresh = QSpinBox(self.groupbox_fileCache)
        self.spinBox_fileCache_evictMaxThresh.setObjectName(u"spinBox_fileCache_evictMaxThresh")
        self.spinBox_fileCache_evictMaxThresh.setMaximum(2147483647)
        self.spinBox_fileCache_evictMaxThresh.setValue(80)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_evictMaxThresh)

        self.spinBox_fileCache_evictMinThresh = QSpinBox(self.groupbox_fileCache)
        self.spinBox_fileCache_evictMinThresh.setObjectName(u"spinBox_fileCache_evictMinThresh")
        self.spinBox_fileCache_evictMinThresh.setMaximum(2147483647)
        self.spinBox_fileCache_evictMinThresh.setValue(60)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_evictMinThresh)

        self.spinBox_fileCache_refreshSec = QSpinBox(self.groupbox_fileCache)
        self.spinBox_fileCache_refreshSec.setObjectName(u"spinBox_fileCache_refreshSec")
        self.spinBox_fileCache_refreshSec.setMaximum(2147483647)
        self.spinBox_fileCache_refreshSec.setSingleStep(20)
        self.spinBox_fileCache_refreshSec.setValue(60)

        self.verticalLayout_7.addWidget(self.spinBox_fileCache_refreshSec)


        self.horizontalLayout_7.addLayout(self.verticalLayout_7)


        self.verticalLayout_9.addLayout(self.horizontalLayout_7)

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


        self.gridLayout.addWidget(self.groupbox_fileCache, 0, 0, 1, 1)

        self.button_resetDefaultSettings = QPushButton(Form)
        self.button_resetDefaultSettings.setObjectName(u"button_resetDefaultSettings")
        self.button_resetDefaultSettings.setEnabled(True)
        self.button_resetDefaultSettings.setMaximumSize(QSize(165, 16777215))
        self.button_resetDefaultSettings.setLayoutDirection(Qt.LeftToRight)

        self.gridLayout.addWidget(self.button_resetDefaultSettings, 3, 0, 1, 1)

        self.groupbox_azure = QGroupBox(Form)
        self.groupbox_azure.setObjectName(u"groupbox_azure")
        self.groupbox_azure.setCheckable(False)
        self.groupbox_azure.setChecked(False)
        self.gridLayout_3 = QGridLayout(self.groupbox_azure)
        self.gridLayout_3.setObjectName(u"gridLayout_3")
        self.horizontalLayout_20 = QHBoxLayout()
        self.horizontalLayout_20.setObjectName(u"horizontalLayout_20")
        self.verticalLayout_15 = QVBoxLayout()
        self.verticalLayout_15.setObjectName(u"verticalLayout_15")
        self.label_30 = QLabel(self.groupbox_azure)
        self.label_30.setObjectName(u"label_30")

        self.verticalLayout_15.addWidget(self.label_30)

        self.label_31 = QLabel(self.groupbox_azure)
        self.label_31.setObjectName(u"label_31")

        self.verticalLayout_15.addWidget(self.label_31)

        self.label_32 = QLabel(self.groupbox_azure)
        self.label_32.setObjectName(u"label_32")

        self.verticalLayout_15.addWidget(self.label_32)

        self.label_33 = QLabel(self.groupbox_azure)
        self.label_33.setObjectName(u"label_33")

        self.verticalLayout_15.addWidget(self.label_33)

        self.label_34 = QLabel(self.groupbox_azure)
        self.label_34.setObjectName(u"label_34")

        self.verticalLayout_15.addWidget(self.label_34)

        self.label_35 = QLabel(self.groupbox_azure)
        self.label_35.setObjectName(u"label_35")

        self.verticalLayout_15.addWidget(self.label_35)

        self.label_36 = QLabel(self.groupbox_azure)
        self.label_36.setObjectName(u"label_36")

        self.verticalLayout_15.addWidget(self.label_36)

        self.label_37 = QLabel(self.groupbox_azure)
        self.label_37.setObjectName(u"label_37")

        self.verticalLayout_15.addWidget(self.label_37)

        self.label_38 = QLabel(self.groupbox_azure)
        self.label_38.setObjectName(u"label_38")

        self.verticalLayout_15.addWidget(self.label_38)

        self.label_39 = QLabel(self.groupbox_azure)
        self.label_39.setObjectName(u"label_39")

        self.verticalLayout_15.addWidget(self.label_39)

        self.label_40 = QLabel(self.groupbox_azure)
        self.label_40.setObjectName(u"label_40")

        self.verticalLayout_15.addWidget(self.label_40)

        self.label_41 = QLabel(self.groupbox_azure)
        self.label_41.setObjectName(u"label_41")

        self.verticalLayout_15.addWidget(self.label_41)

        self.label_42 = QLabel(self.groupbox_azure)
        self.label_42.setObjectName(u"label_42")

        self.verticalLayout_15.addWidget(self.label_42)


        self.horizontalLayout_20.addLayout(self.verticalLayout_15)

        self.verticalLayout_16 = QVBoxLayout()
        self.verticalLayout_16.setObjectName(u"verticalLayout_16")
        self.lineEdit_azure_aadEndpoint = QLineEdit(self.groupbox_azure)
        self.lineEdit_azure_aadEndpoint.setObjectName(u"lineEdit_azure_aadEndpoint")

        self.verticalLayout_16.addWidget(self.lineEdit_azure_aadEndpoint)

        self.lineEdit_azure_subDirectory = QLineEdit(self.groupbox_azure)
        self.lineEdit_azure_subDirectory.setObjectName(u"lineEdit_azure_subDirectory")

        self.verticalLayout_16.addWidget(self.lineEdit_azure_subDirectory)

        self.spinBox_azure_blockSize = QSpinBox(self.groupbox_azure)
        self.spinBox_azure_blockSize.setObjectName(u"spinBox_azure_blockSize")
        self.spinBox_azure_blockSize.setMaximum(2147483647)
        self.spinBox_azure_blockSize.setValue(16)

        self.verticalLayout_16.addWidget(self.spinBox_azure_blockSize)

        self.spinBox_azure_maxConcurrency = QSpinBox(self.groupbox_azure)
        self.spinBox_azure_maxConcurrency.setObjectName(u"spinBox_azure_maxConcurrency")
        self.spinBox_azure_maxConcurrency.setMaximum(2147483647)
        self.spinBox_azure_maxConcurrency.setValue(32)

        self.verticalLayout_16.addWidget(self.spinBox_azure_maxConcurrency)

        self.dropDown_azure_blobTier = QComboBox(self.groupbox_azure)
        self.dropDown_azure_blobTier.addItem("")
        self.dropDown_azure_blobTier.addItem("")
        self.dropDown_azure_blobTier.addItem("")
        self.dropDown_azure_blobTier.addItem("")
        self.dropDown_azure_blobTier.setObjectName(u"dropDown_azure_blobTier")

        self.verticalLayout_16.addWidget(self.dropDown_azure_blobTier)

        self.spinBox_azure_blockOnMount = QSpinBox(self.groupbox_azure)
        self.spinBox_azure_blockOnMount.setObjectName(u"spinBox_azure_blockOnMount")
        self.spinBox_azure_blockOnMount.setMaximum(2147483647)

        self.verticalLayout_16.addWidget(self.spinBox_azure_blockOnMount)

        self.spinBox_azure_maxRetries = QSpinBox(self.groupbox_azure)
        self.spinBox_azure_maxRetries.setObjectName(u"spinBox_azure_maxRetries")
        self.spinBox_azure_maxRetries.setMaximum(2147483647)
        self.spinBox_azure_maxRetries.setValue(5)

        self.verticalLayout_16.addWidget(self.spinBox_azure_maxRetries)

        self.spinBox_azure_maxRetryTimeout = QSpinBox(self.groupbox_azure)
        self.spinBox_azure_maxRetryTimeout.setObjectName(u"spinBox_azure_maxRetryTimeout")
        self.spinBox_azure_maxRetryTimeout.setMaximum(2147483647)
        self.spinBox_azure_maxRetryTimeout.setValue(900)

        self.verticalLayout_16.addWidget(self.spinBox_azure_maxRetryTimeout)

        self.spinBox_azure_retryBackoff = QSpinBox(self.groupbox_azure)
        self.spinBox_azure_retryBackoff.setObjectName(u"spinBox_azure_retryBackoff")
        self.spinBox_azure_retryBackoff.setMaximum(2147483647)
        self.spinBox_azure_retryBackoff.setValue(4)

        self.verticalLayout_16.addWidget(self.spinBox_azure_retryBackoff)

        self.spinBox_azure_maxRetryDelay = QSpinBox(self.groupbox_azure)
        self.spinBox_azure_maxRetryDelay.setObjectName(u"spinBox_azure_maxRetryDelay")
        self.spinBox_azure_maxRetryDelay.setMaximum(2147483647)
        self.spinBox_azure_maxRetryDelay.setValue(60)

        self.verticalLayout_16.addWidget(self.spinBox_azure_maxRetryDelay)

        self.lineEdit_azure_httpProxy = QLineEdit(self.groupbox_azure)
        self.lineEdit_azure_httpProxy.setObjectName(u"lineEdit_azure_httpProxy")

        self.verticalLayout_16.addWidget(self.lineEdit_azure_httpProxy)

        self.lineEdit_azure_httpsProxy = QLineEdit(self.groupbox_azure)
        self.lineEdit_azure_httpsProxy.setObjectName(u"lineEdit_azure_httpsProxy")

        self.verticalLayout_16.addWidget(self.lineEdit_azure_httpsProxy)

        self.lineEdit_azure_authResource = QLineEdit(self.groupbox_azure)
        self.lineEdit_azure_authResource.setObjectName(u"lineEdit_azure_authResource")

        self.verticalLayout_16.addWidget(self.lineEdit_azure_authResource)


        self.horizontalLayout_20.addLayout(self.verticalLayout_16)


        self.gridLayout_3.addLayout(self.horizontalLayout_20, 0, 0, 1, 1)

        self.verticalLayout_18 = QVBoxLayout()
        self.verticalLayout_18.setObjectName(u"verticalLayout_18")
        self.checkBox_azure_useHttp = QCheckBox(self.groupbox_azure)
        self.checkBox_azure_useHttp.setObjectName(u"checkBox_azure_useHttp")

        self.verticalLayout_18.addWidget(self.checkBox_azure_useHttp)

        self.checkBox_azure_validateMd5 = QCheckBox(self.groupbox_azure)
        self.checkBox_azure_validateMd5.setObjectName(u"checkBox_azure_validateMd5")

        self.verticalLayout_18.addWidget(self.checkBox_azure_validateMd5)

        self.checkBox_azure_updateMd5 = QCheckBox(self.groupbox_azure)
        self.checkBox_azure_updateMd5.setObjectName(u"checkBox_azure_updateMd5")

        self.verticalLayout_18.addWidget(self.checkBox_azure_updateMd5)

        self.checkBox_azure_failUnsupportedOps = QCheckBox(self.groupbox_azure)
        self.checkBox_azure_failUnsupportedOps.setObjectName(u"checkBox_azure_failUnsupportedOps")

        self.verticalLayout_18.addWidget(self.checkBox_azure_failUnsupportedOps)

        self.checkBox_azure_sdkTrace = QCheckBox(self.groupbox_azure)
        self.checkBox_azure_sdkTrace.setObjectName(u"checkBox_azure_sdkTrace")

        self.verticalLayout_18.addWidget(self.checkBox_azure_sdkTrace)

        self.checkBox_azure_virtualDirectory = QCheckBox(self.groupbox_azure)
        self.checkBox_azure_virtualDirectory.setObjectName(u"checkBox_azure_virtualDirectory")

        self.verticalLayout_18.addWidget(self.checkBox_azure_virtualDirectory)

        self.checkBox_azure_disableCompression = QCheckBox(self.groupbox_azure)
        self.checkBox_azure_disableCompression.setObjectName(u"checkBox_azure_disableCompression")

        self.verticalLayout_18.addWidget(self.checkBox_azure_disableCompression)


        self.gridLayout_3.addLayout(self.verticalLayout_18, 1, 0, 1, 1)


        self.gridLayout.addWidget(self.groupbox_azure, 0, 1, 2, 1)

        self.button_okay = QPushButton(Form)
        self.button_okay.setObjectName(u"button_okay")
        self.button_okay.setMaximumSize(QSize(100, 16777215))
        self.button_okay.setLayoutDirection(Qt.RightToLeft)

        self.gridLayout.addWidget(self.button_okay, 3, 1, 1, 1)


        self.retranslateUi(Form)

        self.button_resetDefaultSettings.setDefault(False)


        QMetaObject.connectSlotsByName(Form)
    # setupUi

    def retranslateUi(self, Form):
        Form.setWindowTitle(QCoreApplication.translate("Form", u"Form", None))
        self.groupBox_libfuse.setTitle(QCoreApplication.translate("Form", u"LibFuse", None))
#if QT_CONFIG(tooltip)
        self.label.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The number of threads allowed at the libfuse layer for highly parallel operations</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label.setText(QCoreApplication.translate("Form", u"Maximum Fuse Threads", None))
#if QT_CONFIG(tooltip)
        self.checkBox_libfuse_networkshare.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Runs as a network share - may improve performance when latency to cloud is high. </p><p><span style=\" font-weight:600;\">ONLY supported on Windows.</span></p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_libfuse_networkshare.setText(QCoreApplication.translate("Form", u"Enable network-share", None))
#if QT_CONFIG(tooltip)
        self.checkBox_libfuse_disableWriteback.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Dis-allow libfuse to buffer write requests if one must stricty open files in write only or append mode. Alternatively, just set ignore open flags in general settings</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_libfuse_disableWriteback.setText(QCoreApplication.translate("Form", u"Disable write-back cache", None))
        self.groupbox_fileCache.setTitle(QCoreApplication.translate("Form", u"File Caching", None))
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
        self.label_2.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The time (in seconds) between checks to see if a file in the local cache is up to date with the container's latest copy. </p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_2.setText(QCoreApplication.translate("Form", u"Cache Update File Timeout(s)", None))
        self.dropDown_fileCache_evictionPolicy.setItemText(0, QCoreApplication.translate("Form", u"lru - least recently used", None))
        self.dropDown_fileCache_evictionPolicy.setItemText(1, QCoreApplication.translate("Form", u"lfu - least frequently used", None))

#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_allowNonEmptyTmp.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Allow a non-empty temp directory at startup</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_allowNonEmptyTmp.setText(QCoreApplication.translate("Form", u"Allow non-empty temp", None))
#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_policyLogs.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Generate eviction policy logs showing which files will expire soon</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_policyLogs.setText(QCoreApplication.translate("Form", u"Policy logs", None))
#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_createEmptyFile.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Create an empty file on the container when create call is received from the kernel</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_createEmptyFile.setText(QCoreApplication.translate("Form", u"Create empty file", None))
#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_cleanupStart.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Clean the temp directory on startup if it is not empty already</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_cleanupStart.setText(QCoreApplication.translate("Form", u"Cleanup on start", None))
#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_offloadIO.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>By default, the libfuse component will service reads/writes to files for better performance. Check the box to make the file-cache component service read/write calls as well.</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_offloadIO.setText(QCoreApplication.translate("Form", u"Offload IO", None))
#if QT_CONFIG(tooltip)
        self.checkBox_fileCache_syncToFlush.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Sync call to a file will force upload of the contents to the storage account</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_fileCache_syncToFlush.setText(QCoreApplication.translate("Form", u"Sync to flush", None))
#if QT_CONFIG(tooltip)
        self.button_resetDefaultSettings.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Reset to previous settings - does not take effect until changes are saved</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.button_resetDefaultSettings.setText(QCoreApplication.translate("Form", u"Reset Changes", None))
        self.groupbox_azure.setTitle(QCoreApplication.translate("Form", u"Azure Bucket", None))
#if QT_CONFIG(tooltip)
        self.label_30.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Storage account custom AAD endpoint</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_30.setText(QCoreApplication.translate("Form", u"Aad endpoint", None))
#if QT_CONFIG(tooltip)
        self.label_31.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Name of the sub-directory to be mounted instead of the whole container</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_31.setText(QCoreApplication.translate("Form", u"Subdirectory", None))
#if QT_CONFIG(tooltip)
        self.label_32.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The size of each block in MB</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_32.setText(QCoreApplication.translate("Form", u"Block-size (MB)", None))
#if QT_CONFIG(tooltip)
        self.label_33.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Number of parallel upload/download threads</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_33.setText(QCoreApplication.translate("Form", u"Max concurrency", None))
#if QT_CONFIG(tooltip)
        self.label_34.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The 'hot-ness' tier to be set while uploading a blob. </p><p>Default - None</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_34.setText(QCoreApplication.translate("Form", u"Blob tier", None))
#if QT_CONFIG(tooltip)
        self.label_35.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The time (seconds) the list API is blocked after the mount</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_35.setText(QCoreApplication.translate("Form", u"Block on mount (s)", None))
#if QT_CONFIG(tooltip)
        self.label_36.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The number of retries to attempt for any operation failure</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_36.setText(QCoreApplication.translate("Form", u"Max retries", None))
#if QT_CONFIG(tooltip)
        self.label_37.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The maximum time (seconds) allowed for any given retry</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_37.setText(QCoreApplication.translate("Form", u"Max retry timeout (s)", None))
#if QT_CONFIG(tooltip)
        self.label_38.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The minimum amount of time (seconds) to delay between two retries</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_38.setText(QCoreApplication.translate("Form", u"Retry backoff (s)", None))
#if QT_CONFIG(tooltip)
        self.label_39.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The maximum time (seconds) to delay between two retries</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_39.setText(QCoreApplication.translate("Form", u"Max Retry Delay (s)", None))
#if QT_CONFIG(tooltip)
        self.label_40.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>http proxy to be used for connection - [ip-address]:[port]</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_40.setText(QCoreApplication.translate("Form", u"Http proxy", None))
#if QT_CONFIG(tooltip)
        self.label_41.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>https proxy to be used for connection - [ip-address]:[port]</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_41.setText(QCoreApplication.translate("Form", u"Https proxy", None))
#if QT_CONFIG(tooltip)
        self.label_42.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>The resource string to be used during the OAuth token retrieval</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.label_42.setText(QCoreApplication.translate("Form", u"Auth resource", None))
        self.dropDown_azure_blobTier.setItemText(0, QCoreApplication.translate("Form", u"none", None))
        self.dropDown_azure_blobTier.setItemText(1, QCoreApplication.translate("Form", u"hot", None))
        self.dropDown_azure_blobTier.setItemText(2, QCoreApplication.translate("Form", u"cool", None))
        self.dropDown_azure_blobTier.setItemText(3, QCoreApplication.translate("Form", u"archive", None))

#if QT_CONFIG(tooltip)
        self.checkBox_azure_useHttp.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Use http instead of https</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_azure_useHttp.setText(QCoreApplication.translate("Form", u"Use http", None))
#if QT_CONFIG(tooltip)
        self.checkBox_azure_validateMd5.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Validate the md5 on download - this will impact performance and only works when file-cache is enabled in the pipeline</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_azure_validateMd5.setText(QCoreApplication.translate("Form", u"Validate md5 (file cache only)", None))
#if QT_CONFIG(tooltip)
        self.checkBox_azure_updateMd5.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Set the md5 sum to upload. Impacts performance and works only when file-cache is enabled in the pipeline</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_azure_updateMd5.setText(QCoreApplication.translate("Form", u"Update md5 (file cache only)", None))
#if QT_CONFIG(tooltip)
        self.checkBox_azure_failUnsupportedOps.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Return failure for unsupported operations like chmod/chown on block blob accounts</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_azure_failUnsupportedOps.setText(QCoreApplication.translate("Form", u"Fail Unsupported Ops", None))
#if QT_CONFIG(tooltip)
        self.checkBox_azure_sdkTrace.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Enable the storage SDK logging</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_azure_sdkTrace.setText(QCoreApplication.translate("Form", u"Sdk trace", None))
#if QT_CONFIG(tooltip)
        self.checkBox_azure_virtualDirectory.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Support virtual directories without existence of special marker blob</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_azure_virtualDirectory.setText(QCoreApplication.translate("Form", u"Virtual directory", None))
#if QT_CONFIG(tooltip)
        self.checkBox_azure_disableCompression.setToolTip(QCoreApplication.translate("Form", u"<html><head/><body><p>Disable the transport layer content encoding like gzip. Check this flag if blobs have content-encoding set in the container</p></body></html>", None))
#endif // QT_CONFIG(tooltip)
        self.checkBox_azure_disableCompression.setText(QCoreApplication.translate("Form", u"Disable compression", None))
        self.button_okay.setText(QCoreApplication.translate("Form", u"Save", None))
    # retranslateUi

