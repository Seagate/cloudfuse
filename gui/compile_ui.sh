#!/bin/sh -x

rm ui_*.py || true

pyside6-uic mountPrimaryWindow.ui > ui_mountPrimaryWindow.py
pyside6-uic s3_config_common.ui > ui_s3_config_common.py
pyside6-uic azure_config_common.ui > ui_azure_config_common.py
pyside6-uic azure_config_advanced.ui > ui_azure_config_advanced.py
pyside6-uic s3_config_advanced.ui > ui_s3_config_advanced.py
pyside6-uic about_dialog.ui > ui_about_dialog.py
