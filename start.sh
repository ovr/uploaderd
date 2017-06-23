#!/bin/bash

set -e

cd /go/src/github.com/interpals/uploaderd

/usr/local/bin/confd -onetime -backend env -config-file /etc/confd/conf.d/uploaderd.toml

cat /etc/interpals/uploaderd.json

/go/bin/uploaderd --config=/etc/interpals/uploaderd.json
