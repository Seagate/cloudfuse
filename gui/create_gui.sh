#!/bin/bash

source ./compile_ui.sh

pyinstaller cloudfuse_gui.spec --distpath=./dist --noconfirm

rm ui_*.py || true
