# Installing as a Linux systemd service

This is useful if you want to run the hub or agent in the background continuously, including after a reboot.

## Install script (recommended)

There are two scripts, one for the hub and one for the agent. You can run either one, or both.

The install script creates a dedicated user for the service (`aether`), downloads the latest release from the legacy Beszel source, and installs the service.

If you need to edit the service -- for instance, to change an environment variable -- you can edit the file(s) in `/etc/systemd/system/`. Then reload the systemd daemon and restart the service.

> [!NOTE]
> You need system administrator privileges to run the install script. If you encounter a problem, please open an issue in this repository (legacy Beszel issue links are deprecated).

### Hub

Download the script:

```bash
curl -sL https://raw.githubusercontent.com/LuBoyanzy/Aether/main/supplemental/scripts/install-hub.sh -o install-hub.sh && chmod +x install-hub.sh
```

#### Install

You may specify a port number with the `-p` flag. The default port is `19090`.

```bash
./install-hub.sh
```

#### Uninstall

```bash
./install-hub.sh -u
```

#### Update

```bash
sudo /opt/aether/aether update && sudo systemctl restart aether-hub
```

### Agent

Download the script:

```bash
curl -sL https://raw.githubusercontent.com/LuBoyanzy/Aether/main/supplemental/scripts/install-agent.sh -o install-agent.sh && chmod +x install-agent.sh
```

#### Install

You may optionally include the SSH key and port as arguments. Run `./install-agent.sh -h` for more info.

If specifying your key with `-k`, please make sure to enclose it in quotes.

```bash
./install-agent.sh
```

#### Uninstall

```bash
./install-agent.sh -u
```

#### Update

```bash
sudo /opt/aether-agent/aether-agent update && sudo systemctl restart aether-agent
```

## Manual install

### Hub

1. Create the system service at `/etc/systemd/system/aether.service`

```bash
[Unit]
Description=Aether Hub Service
After=network.target

[Service]
# update the values in the curly braces below (remove the braces)
ExecStart={/path/to/working/directory}/aether serve
WorkingDirectory={/path/to/working/directory}
User={YOUR_USERNAME}
Restart=always

[Install]
WantedBy=multi-user.target
```

2. Start and enable the service to let it run after system boot

```bash
sudo systemctl daemon-reload
sudo systemctl enable aether.service
sudo systemctl start aether.service
```

### Agent

1. Create the system service at `/etc/systemd/system/aether-agent.service`

```bash
[Unit]
Description=Aether Agent Service
After=network.target

[Service]
# update the values in curly braces below (remove the braces)
Environment="PORT={PASTE_YOUR_PORT_HERE}"
Environment="KEY={PASTE_YOUR_KEY_HERE}"
# Environment="EXTRA_FILESYSTEMS={sdb}"
ExecStart={/path/to/directory}/aether-agent
User={YOUR_USERNAME}
Restart=always

[Install]
WantedBy=multi-user.target
```

2. Start and enable the service to let it run after system boot

```bash
sudo systemctl daemon-reload
sudo systemctl enable aether-agent.service
sudo systemctl start aether-agent.service
```
