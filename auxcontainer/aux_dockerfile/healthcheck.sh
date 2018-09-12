#!/bin/sh

netstat -ltnu | grep 21 || exit 1
netstat -ltnu | grep 80 || exit 1
echo "ls" | nc localhost 21 || exit 1
echo "GET / HTTP/1.1" | nc localhost 80 || exit 1
# netstat -ltnu | grep 53 || exit 1