#!/bin/sh
DOMAIN=$1

echo "Post-hook triggered at $(date '+%Y-%m-%d %H:%M:%S')"
chmod 644 /certs/turn.${DOMAIN}/privatekey.key

if [ $? -eq 0 ]; then
    echo "Successfully updated permissions"
else
    echo "Failed to update permissions"
    exit 1
fi