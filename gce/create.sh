#! /bin/bash
gcloud compute instances create sclh-instance \
       --image-family=debian-8 \
       --image-project=debian-cloud \
       --machine-type=f1-micro \
       --scopes userinfo-email,cloud-platform \
       --metadata-from-file startup-script=gce/startup-script.sh \
       --zone us-east1-b \
       --tags http-server
