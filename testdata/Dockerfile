FROM node:18

RUN apt-get update && \
  apt-get install -y \
    gnupg \
    curl

COPY pub.txt /key.txt
RUN mkdir -p /usr/local/share/keyrings && \
   gpg --no-default-keyring --keyring /usr/local/share/keyrings/hedge-test.gpg --import < key.txt

RUN echo "deb [signed-by=/usr/local/share/keyrings/hedge-test.gpg] http://192.168.1.23:8080/debian bullseye main" > /etc/apt/sources.list
# RUN echo "deb [signed-by=/usr/local/share/keyrings/hedge-test.gpg] http://192.168.1.23:8080/debian github main" >> /etc/apt/sources.list
RUN npm set registry http://192.168.1.23:8080/npm/stable

COPY package.json /app/package.json
WORKDIR /app

ENTRYPOINT ["/bin/bash"]
