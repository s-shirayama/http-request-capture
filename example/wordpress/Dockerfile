FROM wordpress:latest

RUN sed -i 's/Listen 80$/Listen 8000/' /etc/apache2/ports.conf \
 && sed -i 's/:80>/:8000>/' /etc/apache2/sites-available/000-default.conf
