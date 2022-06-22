#! /bin/bash

# <UDF name="github_org" label="GitHub Org" />
# <UDF name="runner_cfg_pat" label="GitHub Personal Token" />

set -x

apt-get update
apt-get install -y --no-install-recommends apt-transport-https ca-certificates curl jq

export USER=runner

# Create GitHun Runner user
adduser --disabled-password --gecos "" $USER
usermod -aG sudo $USER
# newgrp sudo
rsync --archive --chown=$USER:$USER ~/.ssh /home/$USER
echo "$USER ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers
# sudo usermod -aG docker $USER

# Install GitHub Runner
su $USER
cd /home/$USER

# https://github.com/actions/runner/blob/main/docs/automate.md
curl -s https://raw.githubusercontent.com/actions/runner/main/scripts/create-latest-svc.sh | bash -s -- -s ${GITHUB_ORG} -n linode-${LINODE_ID} -l ubuntu-latest
