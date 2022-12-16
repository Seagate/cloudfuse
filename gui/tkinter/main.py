import tkinter as tk
from tkinter import filedialog
from tkinter import messagebox
from tkinter import ttk

import subprocess

class LyveCloudFuse:

  entry_width = 40

  def __init__(self, root):
    root.title("LyveCloudFuse")

    mainframe = ttk.Frame(root, padding="3 3 25 25")
    mainframe.grid(column=0, row=0, sticky=(tk.N, tk.W, tk.E, tk.S))
    root.columnconfigure(0, weight=1)
    root.rowconfigure(0, weight=1)

    ttk.Label(mainframe, text="Mount Directory").grid(column=1, row=1, sticky=tk.W)
    self.mount_directory = tk.StringVar()
    mount_directory_entry = ttk.Entry(mainframe, width = self.entry_width, textvariable = self.mount_directory)
    mount_directory_entry.grid(column=2, row=1, sticky=(tk.W,tk.E), columnspan=4)
    ttk.Button(mainframe, text="Browse", command=self.browse_button).grid(column=7, row=1, sticky=(tk.W,tk.E))
    
    ttk.Button(mainframe, text="Mount", command=self.mount_bucket).grid(column=2, row=6, sticky=tk.W)
    ttk.Button(mainframe, text="Unmount", command=self.unmount_bucket).grid(column=3, row=6, sticky=tk.W)

    for child in mainframe.winfo_children(): 
      child.grid_configure(padx=5, pady=5)

    mount_directory_entry.focus()
    root.bind("<Return>", self.mount_bucket)
  
  def browse_button(self):
    filename = filedialog.askdirectory()
    self.mount_directory.set(filename)
    print(filename)

  def error_window(self, message):
    messagebox.showerror("Message", message)

  def success_window(self, message):
    messagebox.showinfo("Message", message)

  def mount_bucket(self, *args):
    try:
      mount_directory = str(self.mount_directory.get())

      mount = subprocess.run(["./azure-storage-fuse", "mount", "all", mount_directory, "--config-file=./config.yaml"])
      if mount.returncode == 0:
        self.success_window("Successfully mounted container")
      else:
        self.error_window("Error mounting container")

    except ValueError:
      pass
    
  def unmount_bucket(self, *args):
    try:
      umount = subprocess.run(["./azure-storage-fuse", "unmount", "all"])
      if umount.returncode == 0:
        self.success_window("Successfully unmounted container")
      else:
        self.error_window("Error unmounting container")

    except ValueError:
      pass
    
root = tk.Tk()
LyveCloudFuse(root)
root.mainloop()
