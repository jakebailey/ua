FROM pygmy/alpine-tini
MAINTAINER Jake Bailey <jbbaile2@illinois.edu>

RUN echo "Hello!" >> /hello.txt

CMD /bin/sh