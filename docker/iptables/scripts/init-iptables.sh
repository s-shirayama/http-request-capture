#!/bin/sh

# Delete existing one (if they are there)
iptables -t nat -D OUTPUT -p tcp -j ISTIO_ROUTE 2> /dev/null
iptables -t nat -F ISTIO_ROUTE 2> /dev/null
iptables -t nat -X ISTIO_ROUTE 2> /dev/null
iptables -t nat -F ISTIO_OUTPUT 2> /dev/null
iptables -t nat -X ISTIO_OUTPUT 2> /dev/null
iptables -t nat -F ISTIO_REDIRECT 2> /dev/null
iptables -t nat -X ISTIO_REDIRECT 2> /dev/null

# Add routing for proxy
iptables -t nat -N ISTIO_REDIRECT
iptables -t nat -N ISTIO_OUTPUT
iptables -t nat -N ISTIO_ROUTE

CAPTURE_TARGET_PORT="80,443"
DOCKER_HOST_IP=`host docker.for.mac.host.internal | sed -n 1P | cut -d ' ' -f 4`
if [ "${PROXY_PORT}" != "" ]; then
  CAPTURE_TARGET_PORT="${CAPTURE_TARGET_PORT},${PROXY_PORT}"
  iptables -t nat -A ISTIO_REDIRECT -p tcp --dport ${PROXY_PORT} -j DNAT --to-destination ${DOCKER_HOST_IP}:18083
fi
iptables -t nat -A ISTIO_REDIRECT -p tcp --dport 80  -j DNAT --to-destination ${DOCKER_HOST_IP}:18080
iptables -t nat -A ISTIO_REDIRECT -p tcp --dport 443 -j DNAT --to-destination ${DOCKER_HOST_IP}:18082
iptables -t nat -A ISTIO_OUTPUT -o lo ! -d 127.0.0.1/32 -j ISTIO_REDIRECT
iptables -t nat -A ISTIO_OUTPUT -d 127.0.0.1/32 -j RETURN
iptables -t nat -A ISTIO_OUTPUT -j ISTIO_REDIRECT
iptables -t nat -A ISTIO_ROUTE -p tcp --match multiport --dports ${CAPTURE_TARGET_PORT} -j ISTIO_OUTPUT
iptables -t nat -A OUTPUT -p tcp -j ISTIO_ROUTE

# Confirm current routing
iptables -t nat -n -L -v

# Set termination process when docker-compose stop/down
trap_TERM() {
  sh /etc/scripts/reset-iptables.sh
  exit 0
}
trap 'trap_TERM' TERM

while :
do
  sleep 1
done
