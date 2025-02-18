"""
Common utility functions
"""

# Licensed under the MIT License <http://opensource.org/licenses/MIT>.
#
# Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE

from sys import platform

from PySide6.QtGui import QRegularExpressionValidator
from PySide6 import QtWidgets

# noinspection PyUnresolvedReferences
from __feature__ import snake_case, true_property


def set_path_validator(widget: QtWidgets):
    """
    Set regex for path validators for the current operating system.

    Args:
        widget (QtWidgets): Widget to set regex.
    """
    if platform == 'win32':
        # Windows directory and filename conventions:
        # https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#file-and-directory-names
        # Disallow the following [<,>,.,",|,?,*] - note, we still need directory characters to declare a path
        regex = r'^[^<>."|?\0*]*$'
    else:
        # Allow anything BUT Nul
        # Note: Different versions of Python don't like the embedded null character, send in the raw string instead
        regex = r'^[^\0]*$'
    widget.set_validator(QRegularExpressionValidator(regex, widget))

def populate_widgets_from_settings(mapping: dict, settings: dict):
    """
    Set the appropriate widget corresponding to each setting

    Args:
        mapping (dict): mapping from setting name to the corresponding widget.
        settings (dict): mapping from setting name to the current value.
    """
    for key, widget in mapping.items():
        value = settings.get(key)
        if hasattr(widget, 'setChecked'):
            widget.set_checked(bool(value))
        elif hasattr(widget, 'setText'):
            widget.set_text(str(value))
        elif hasattr(widget, 'setValue'):
            widget.set_value(int(value))

def update_settings_from_widgets(mapping: dict, settings: dict):
    """
    Set the values in settings from the values the user selected in the ui.

    Args:
        mapping (dict): mapping from setting name to the corresponding widget.
        settings (dict): mapping from setting name to the current value.
    """
    for key, widget in mapping.items():
        if hasattr(widget, 'isChecked'):
            settings[key] = widget.is_checked()
        elif hasattr(widget, 'currentIndex'):
            settings[key] = widget.current_index()
        elif hasattr(widget, 'text'):
            settings[key] = widget.text()
        elif hasattr(widget, 'value'):
            settings[key] = widget.value()
