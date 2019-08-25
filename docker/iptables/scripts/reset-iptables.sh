#!/bin/sh

# Delete existing iptables setttings
iptables -t nat -D OUTPUT -p tcp -j ISTIO_ROUTE 2> /dev/null
iptables -t nat -F ISTIO_ROUTE 2> /dev/null
iptables -t nat -X ISTIO_ROUTE 2> /dev/null
iptables -t nat -F ISTIO_OUTPUT 2> /dev/null
iptables -t nat -X ISTIO_OUTPUT 2> /dev/null
iptables -t nat -F ISTIO_REDIRECT 2> /dev/null
iptables -t nat -X ISTIO_REDIRECT 2> /dev/null

# Confirm current routing
iptables -t nat -n -L -v
