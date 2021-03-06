FROM alpine:3.7

RUN apk update && \
    apk add --no-cache bash==4.4.19-r1 \
    vsftpd==3.0.3-r4 unbound==1.6.7-r1 \
    nginx==1.12.2-r3 nano


RUN apk add fuse

RUN apk add libmagic

RUN apk add supervisor

RUN apk add python3 && \
    python3 -m ensurepip && \
    rm -r /usr/lib/python*/ensurepip && \
    pip3 install --upgrade ramrodbrain

RUN pip3 install fusepy==2.0.4

RUN pip3 install flask

COPY ./supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Configure openrc
RUN mkdir /run/openrc && \
    touch /run/openrc/softlevel

# Set up Nginx
RUN adduser -D -g 'www' www && \
    mkdir -p /www/files && \
    mkdir -p /run/nginx && \
    chown -R www:www /var/lib/nginx && \
    chown -R www:www /www

COPY ./config/nginx/nginx.conf /etc/nginx/nginx.conf
COPY ./config/nginx/index.html /www/index.html

# Set up vsftpd
COPY ./config/vsftpd/vsftpd.conf /etc/vsftpd/vsftpd.conf

RUN adduser -D -g 'fff' fff
RUN mkdir /home/fff/files
RUN chown fff /home/fff/files
RUN mkdir /home/fff/local
RUN chown fff /home/fff/local
RUN chmod 700 /home/fff/local
RUN chmod 500 /home/fff

COPY ./config/run.sh /run.sh
COPY ./config/brain_filesystem.py /brain_filesystem.py
COPY ./config/upload/uploads.py /uploads.py
COPY ./healthcheck.sh .

HEALTHCHECK --interval=2s --start-period=10s --retries=3 CMD /bin/sh ./healthcheck.sh

EXPOSE 20 21 80 10090-10100

ENTRYPOINT [ "/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]