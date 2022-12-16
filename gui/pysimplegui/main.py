import PySimpleGUI as sg
import subprocess

def mount_bucket(mount_directory):
    try:
      mount = subprocess.run(["./azure-storage-fuse", "mount", "all", mount_directory, "--config-file=./config.yaml"])
      if mount.returncode == 0:
        sg.Popup("Successfully mounted container", keep_on_top=True)
      else:
        sg.Popup("Error mounting container", keep_on_top=True)

    except ValueError:
      pass

def unmount_bucket():
    try:
        umount = subprocess.run(["./azure-storage-fuse", "unmount", "all"])
        if umount.returncode == 0:
            sg.Popup("Successfully unmounted container", keep_on_top=True)
        else:
            sg.Popup("Error unmounting container", keep_on_top=True)
    except ValueError:
        pass

layout = [[sg.Text('Settings'),
          sg.In(size=(25,1), enable_events=True ,key='-FOLDER-'),
          sg.FolderBrowse()],
          [sg.Button('Mount'), sg.Button('Unmount')]]

window = sg.Window('LyveCloudFUSE Demo PysimpleGUI', layout)

while True:
    event, values = window.read()

    if event == sg.WIN_CLOSED or event == 'Exit':
        break
    if event == 'Mount':
        # sg.user_settings_set_entry('-filenames-', list(set(sg.user_settings_get_entry('-filenames-', []) + [values['-FILENAME-'], ])))
        # sg.user_settings_set_entry('-last filename-', values['-FILENAME-'])
        # window['-FILENAME-'].update(values=list(set(sg.user_settings_get_entry('-filenames-', []))))
        mount_directory = values['-FOLDER-']
        mount_bucket(mount_directory)
    elif event == 'Unmount':
        # sg.user_settings_set_entry('-filenames-', [])
        # sg.user_settings_set_entry('-last filename-', '')
        # window['-FILENAME-'].update(values=[], value='')
        unmount_bucket()
