#!/bin/sh -x

rm ui*.py || true

pyside6-uic mountPrimaryWindow.ui > ui_mountPrimaryWindow.py
pyside6-uic lyve_config_common.ui > ui_lyve_config_common.py
pyside6-uic azure_config_common.ui > ui_azure_config_common.py
pyside6-uic azure_config_advanced.ui > ui_azure_config_advanced.py
pyside6-uic lyve_config_advanced.ui > ui_lyve_config_advanced.py