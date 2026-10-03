#!/bin/bash
# =============================================================================
# GOSHOP - LET'S ENCRYPT SSL BOOTSTRAP SCRIPT
# =============================================================================
# Senior System Designer Guide:
# This script handles the initial chicken-and-egg problem where Nginx needs SSL
# certificates to start, but Certbot needs Nginx running to pass the HTTP-01 challenge.
# =============================================================================

set -e

DOMAINS=("shop.example.com")
RSA_KEY_SIZE=4096
DATA_PATH="./certbot"
EMAIL="stl@gmail.com" # Thay bằng email quản trị viên
STAGING=0 # Đặt = 1 khi muốn test tránh bị rate limit của Let's Encrypt

if [ -d "$DATA_PATH/conf/live/${DOMAINS[0]}" ]; then
  read -p "Existing certificate data found for ${DOMAINS[0]}. Continue and overwrite? (y/N) " decision
  if [ "$decision" != "Y" ] && [ "$decision" != "y" ]; then
    exit
  fi
fi

echo "### Downloading recommended TLS parameters ..."
mkdir -p "$DATA_PATH/conf"
curl -s https://raw.githubusercontent.com/certbot/certbot/master/certbot-nginx/certbot_nginx/_internal/tls_configs/options-ssl-nginx.conf > "$DATA_PATH/conf/options-ssl-nginx.conf"
curl -s https://raw.githubusercontent.com/certbot/certbot/master/certbot/certbot/ssl-dhparams.pem > "$DATA_PATH/conf/ssl-dhparams.pem"

echo "### Creating dummy certificate for ${DOMAINS[0]} ..."
path="/etc/letsencrypt/live/${DOMAINS[0]}"
mkdir -p "$DATA_PATH/conf/live/${DOMAINS[0]}"
docker compose -f docker-compose.prod.yml run --rm --entrypoint "\
  openssl req -x509 -nodes -newkey rsa:$RSA_KEY_SIZE -days 1\
    -keyout '$path/privkey.pem' \
    -out '$path/fullchain.pem' \
    -subj '/CN=localhost'" certbot

echo "### Starting Nginx ..."
docker compose -f docker-compose.prod.yml up --force-recreate -d nginx

echo "### Deleting dummy certificate for ${DOMAINS[0]} ..."
docker compose -f docker-compose.prod.yml run --rm --entrypoint "\
  rm -Rf /etc/letsencrypt/live/${DOMAINS[0]} && \
  rm -Rf /etc/letsencrypt/archive/${DOMAINS[0]} && \
  rm -Rf /etc/letsencrypt/renewal/${DOMAINS[0]}.conf" certbot

echo "### Requesting Let's Encrypt certificate for ${DOMAINS[*]} ..."
domain_args=""
for domain in "${DOMAINS[@]}"; do
  domain_args="$domain_args -d $domain"
done

# Select appropriate email arg
case "$EMAIL" in
  "") email_arg="--register-unsafely-without-email" ;;
  *) email_arg="--email $EMAIL" ;;
esac

# Enable staging mode if needed
if [ $STAGING != "0" ]; then staging_arg="--staging"; fi

docker compose -f docker-compose.prod.yml run --rm --entrypoint "\
  certbot certonly --webroot -w /var/www/certbot \
    $staging_arg \
    $email_arg \
    $domain_args \
    --rsa-key-size $RSA_KEY_SIZE \
    --agree-tos \
    --force-renewal" certbot

echo "### Reloading Nginx with new certificate ..."
docker compose -f docker-compose.prod.yml exec nginx nginx -s reload

echo "=== SSL Certificate successfully provisioned! ==="
