#!/bin/sh
echo "network parameters: delay:${DELAY} loss:${LOSS}"
tc qdisc add dev eth0 root netem delay ${DELAY} loss ${LOSS}
exec "$@"
