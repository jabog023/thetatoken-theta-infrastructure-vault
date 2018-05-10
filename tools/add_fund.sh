#!/bin/bash

set -e

pass="qwertyuiop"

seq="$(thetacli query account 6D2F435CF553C111F23D120CA24D794ADB327738 | jq -r '.data.sequence')"
((seq++))
echo "seq: $seq\n"
/usr/bin/expect <<EOD
spawn thetacli tx send --name=faucet --amount=$2ThetaWei --to=$1 --sequence=$seq
expect "Please enter passphrase"
send "$pass\r"
expect "hash"
interact
EOD

