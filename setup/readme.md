# systemd service file for cloudfuse

## Steps to install Cloudfuse to systemd

1. Prepare the configuration file `config.yaml` that cloudfuse will use to start your mount. You may follow the instructions for creating a config.yaml file in the [Readme](../README.md#basic-use) or by creating the config file manually by following the [Wiki](https://github.com/Seagate/cloudfuse/wiki/Config-File).
2. In the cloudfuse.service file, edit all of the fields under the `Service` section and replace them for your system. Please note that the example has the User CloudfuseUser, please create a user called CloudfuseUser or replace this with an existing user.
3. Copy the cloudfuse.service and place it in /etc/systemd/system:
   `sudo cp cloudfuse.service /etc/systemd/system`
4. Run the daemon-reload command to reload the service config files:
   `sudo systemctl daemon-reload`
5. Start the service:
   `sudo systemctl start cloudfuse.service`
6. Enable the service to start at system boot:
   `sudo systemctl enable cloudfuse.service`
