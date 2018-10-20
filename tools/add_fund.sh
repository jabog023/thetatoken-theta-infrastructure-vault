#!/bin/bash

set -e

pass="qwertyuiop"
seq=$(banjo query account --address=$1 | jq -r '.sequence')
((seq++))
echo "seq: $seq\n"
/usr/bin/expect <<EOD
spawn banjo tx send --chain= --from=$1 --to=$2 --theta=$3 --gamma=$4 --seq=$5
expect "Please enter password"
send "$pass\r"
expect "transaction"
interact
EOD