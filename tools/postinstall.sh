#!/bin/bash
# This script is packaged with the .deb or .rpm package and executed as a post install script.
# It setups up the autocompletion scripts for various shells depending upon it's existence in the system

# Autocompletions for bash shell
echo "Generating bash autocompletes........."
cloudfuse completion bash >/etc/bash_completion.d/cloudfuse

for user in $(getent passwd {1000..60000} | cut -d: -f1); do
  home=$(eval echo "~${user}")
  # Autocompletes for zsh shell
  if grep -q "zsh" /etc/shells; then
    echo "Found zsh..... Generating autocompletes......"
    cloudfuse completion zsh >"${home}"/.oh-my-zsh/custom/plugins/zsh-autosuggestions/_cloudfuse
  fi
  # Autocompletes for fish shell
  if grep -q "fish" /etc/shells; then
    echo "Found fish..... Generating autocompletes"
    cloudfuse completion fish >"${home}".config/fish/completions/cloudfuse.fish
  fi
done

echo "Finished shell autocompletes!"

if [ -d "/etc/rsyslog.d/" ]
then
  echo "Configuring syslog......."
  sudo service rsyslog restart
  echo "Finished syslog configuration!"
fi
