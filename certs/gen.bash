#!/usr/bin/env bash
set -e

openssl req -new -nodes -x509 -out ./server.pem -keyout ./server.key -days 18250 -subj "/C=RU/ST=WIRENET/L=Earth/O=mediabuyerbot/CN=mediabuyerbot.com/emailAddress=mediabuyerbot@gmail.com"
openssl req -new -nodes -x509 -out ./client.pem -keyout ./client.key -days 18250 -subj "/C=RU/ST=WIRENET/L=Earth/O=mediabuyerbot/CN=mediabuyerbot.com/emailAddress=mediabuyerbot@gmail.com"

