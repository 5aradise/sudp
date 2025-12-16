#!/bin/sh
echo "network parameters: delay:${DELAY:-0ms} loss:${LOSS:-0%}"
tc qdisc add dev eth0 root netem delay ${DELAY:-0ms} loss ${LOSS:-0%}
exec "$@"
