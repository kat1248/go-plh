#! /bin/bash

SCLH_DEPLOY_LOCATION=gs://sclh-deploy/go-plh.tar

gsutil cp $SCLH_DEPLOY_LOCATION /app.tar
mkdir -p /app
tar -x -f /app.tar -C /app
chmod +x /app/go-plh

setcap CAP_NET_BIND_SERVICE=+eip /app/go-plh

# restart the process
supervisorctl restart goapp