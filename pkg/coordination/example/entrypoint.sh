#!/bin/sh

# POD_IP: the container's routable IP, advertised to NATS cluster peers
# (see Dockerfile ENTRYPOINT). Compose sets it automatically unless one was
# provided, so replicas discover each other inside the Docker network.
if [ -z "${POD_IP:-}" ]; then
	POD_IP=$(hostname -i | awk '{print $1}')
	export POD_IP
fi

exec /app/coordinator
