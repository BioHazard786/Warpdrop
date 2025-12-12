#!/bin/sh
echo "Post-hook triggered at $(date '+%Y-%m-%d %H:%M:%S')"
chmod 644 /certs/${DOMAIN}/privatekey.key
if [ $? -eq 0 ]; then
    echo "Successfully updated permissions"
else
    echo "Failed to update permissions"
    exit 1
fi