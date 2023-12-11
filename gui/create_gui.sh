#!/bin/bash

source ./compile_ui.sh

pyinstaller cloudfuse_gui.spec --distpath=./dist

rm ui_*.py || true