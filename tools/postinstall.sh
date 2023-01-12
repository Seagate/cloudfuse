#!/bin/bash
# This script is packaged with the .deb or .rpm package and executed as a post install script.
# It setups up the autocompletion scripts for various shells depending upon it's existence in the system

# Autocompletions for bash shell
echo "Generating bash autocompletes........."
lyvecloudfuse completion bash >/etc/bash_completion.d/lyvecloudfuse

for user in $(getent passwd {1000..60000} | cut -d: -f1); do
  home=$(eval echo "~${user}")
  # Autocompletes for zsh shell
  if cat /etc/shells | grep -q "zsh"; then
    echo "Found zsh..... Generating autocompletes......"
    lyvecloudfuse completion zsh >"${home}"/.oh-my-zsh/custom/plugins/zsh-autosuggestions/_lyvecloudfuse
  fi
  # Autocompletes for fish shell
  if cat /etc/shells | grep -q "fish"; then
    echo "Found fish..... Generating autocompletes"
    lyvecloudfuse completion fish >"${home}".config/fish/completions/lyvecloudfuse.fish
  fi
done

echo "Finished shell autocompletes!"

if [ -d "/etc/rsyslog.d/" ]
then
  echo "Configuring syslog......."
  sudo service rsyslog restart
  echo "Finished syslog configuration!"
fi
