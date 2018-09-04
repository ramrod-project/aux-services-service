#!/bin/bash

watch python3 uploads.py &

# start nginx
su www -c "python3 /brain_filesystem.py /www/files" &
sleep 2
nginx -g 'daemon off;' &

# start vsftpd
su fff -c "python3 /brain_filesystem.py /home/fff/files" &
sleep 2
/usr/sbin/vsftpd /etc/vsftpd/vsftpd.conf