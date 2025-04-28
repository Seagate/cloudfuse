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

from PySide6.QtWidgets import QMessageBox
from PySide6.QtCore import Qt

class aboutPage(QMessageBox):
    def __init__(self, cloudfuseVersion: str):
        super().__init__()
        self.setWindowTitle('About Cloudfuse')
        self.setTextFormat(Qt.RichText)
        self.setText(f"""
            <p><strong><h3>About Cloudfuse</h3></strong></p>
            <p>This program is using Cloudfuse version {cloudfuseVersion}</p>
            <p>Cloudfuse provides a virtual filesystem backed by either S3 or Azure Storage for mounting the cloud to a local system.</p>
            <p>For more information and frequently asked questions please go to the <a href="https://github.com/Seagate/cloudfuse">Cloudfuse GitHub</a>.</p>
            <p><h4>Help</h4></p>
            <p>Found a bug? Report it at <a href="https://github.com/Seagate/cloudfuse/issues">github issues</a> with the bug label.</p>
            <p>Have questions not in the FAQ? Submit it at <a href="https://github.com/Seagate/cloudfuse/issues">github issues</a> with the question label.</p>
            <p><h4>Third Party Notices</h4></p>
            <p>See <a href="NOTICE">notices</a> for third party license notices.</p>
            <p>WinSFP is licensed under the GPLv3 license with a special exception for Free/Libre and Open Source Software, which is available <a href="https://github.com/winfsp/winfsp/blob/master/License.txt">here</a>.</p>
            <p><h4>Attribution</h4></p>
            <p>WinSFP - Windows File System Proxy, Copyright Bill Zissimopoulos <a href="https://github.com/winfsp/winfsp">details</a>.</p>
            <p><h4>License</h4></p>
            <p>This project is licensed under MIT.</p>
        """)
