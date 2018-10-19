#!/bin/bash

set -e

pass="qwertyuiop"
faucet="2E833968E5bB786Ae419c4d13189fB081Cc43bab"

seq=$(banjo query account --address=$faucet | jq -r '.sequence')
((seq++))
echo "seq: $seq\n"
/usr/bin/expect <<EOD
spawn banjo tx send --chain= --from=$faucet --to=$1 --theta=$2 --gamma=$3 --seq=$seq
expect "Please enter password"
send "$pass\r"
expect "transaction"
interact
EOD

