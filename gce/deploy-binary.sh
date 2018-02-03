#! /bin/bash

# Copyright 2015 Google Inc. All rights reserved.
# Use of this source code is governed by the Apache 2.0
# license that can be found in the LICENSE file.

set -ex

if [ ! $(dirname $0) = "." ]; then
  echo "Must run $(basename $0) from the gce_deployment directory."
  exit 1
fi

SCLH_DEPLOY_LOCATION=gs://sclh-deploy/go-plh.tar

if [ -z "$SCLH_DEPLOY_LOCATION" ]; then
  echo "Must set \$SCLH_DEPLOY_LOCATION. For example: SCLH_DEPLOY_LOCATION=gs://my-bucket/bookshelf-VERSION.tar"
  exit 1
fi

TMP=$(mktemp -d -t gce-deploy-XXXXXX)

# [START cross_compile]
# Cross compile the app for linux/amd64
GOOS=linux GOARCH=amd64 go build -v -o $TMP/go-plh ../go-plh
# [END cross_compile]

# [START tar]
# Add the app binary
tar -c -f $TMP/bundle.tar -C $TMP go-plh

# Add static files.
tar -u -f $TMP/bundle.tar -C ../go-plh templates
tar -u -f $TMP/bundle.tar -C ../go-plh static
# [END tar]

# [START gcs_push]
# SCLH_DEPLOY_LOCATION is something like "gs://my-bucket/bookshelf-VERSION.tar".
gsutil cp $TMP/bundle.tar $SCLH_DEPLOY_LOCATION
# [END gcs_push]

rm -rf $TMP
