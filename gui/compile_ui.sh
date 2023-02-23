#!/bin/sh

rm ui*.py

pyside6-uic mountPrimaryWindow.ui > ui_mountPrimaryWindow.py
pyside6-uic AzModeOptions.ui > ui_AzModeOptions.py
pyside6-uic FCAz_simpleSettings.ui > ui_FCAz_simpleSettings.py
pyside6-uic StAz_simpleSettings.ui > ui_StAz_simpleSettings.py
pyside6-uic FCL_simpleSettings.ui > ui_FCL_simpleSettings.py
pyside6-uic StL_simpleSettings.ui > ui_StL_simpleSettings.py
pyside6-uic mountSettings.ui > ui_mountSettings.py