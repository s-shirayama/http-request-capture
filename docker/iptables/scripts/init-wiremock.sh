#!/bin/sh

for target_url in `echo ${PROXY_TARGET_URL} | tr ' ' '\n'`
do
  curl docker.for.mac.host.internal:18083/__admin/mappings -d '{
    "request": {
      "headers": {
        "Host": {
          "equalTo": "'`echo ${target_url} | cut -d '/' -f 3`'"
        }
      },
      "method": "ANY"
    },
    "response" : {
      "status": 200,
      "proxyBaseUrl" : "'${target_url}'"
    },
    "priority": 10
  }'
done
