# systemd service file for cloudfuse

## Steps to install Cloudfuse to systemd

1. Prepare the configuration file `config.yaml` that cloudfuse will use to start your mount. Follow the general instructions for creating a config.yaml file from the Readme.
2. In the cloudfuse.service file, edit all of the fields under the `Service` section and replace them for your system. Please note that the example has the User CloudfuseUser, please create a user called CloudfuseUser or replace this with an existing user.
3. Copy the cloudfuse.service and place it in /etc/systemd/system:
   `cp cloudfuse.service /etc/systemd/system`
4. Run the daemon-reload command to reload the service config files:
   `systemctl daemon-reload`
5. Start the service:
   `systemctl start cloudfuse.service`
6. Enable the service to start at system boot:
   `systemctl enable cloudfuse.service`
