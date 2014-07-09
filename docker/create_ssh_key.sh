#!/bin/bash
if [ -z ${SSH_KEY} ]
then
 echo "No SSH_KEY environment variable set; you will be unable to login."
 exit 1
else
     echo "Copying SSH_KEY to authorized_keys for root."
     echo ${SSH_KEY} > /root/.ssh/authorized_keys
     exit 0
fi
