## Docker Grand Ambassador [![Docker Build Status](http://hubstatus.container42.com/cpuguy83/docker-grand-ambassador)](https://registry.hub.docker.com/u/cpuguy83/docker-grand-ambassador)

This is a fully dynamic docker link ambassador.  

For information on link ambassadors see:
http://docs.docker.com/articles/ambassador_pattern_linking/


### Problem
The problem with linking is that links are static.  When a container which is
being linked to is restarted it very likely has a new IP address.  Any container
which is linked to this restarted container will also need to be restarted in
order to pick up this new IP address.  Therefore linked containers can often
have a cascading effect of needing to restart many containers in order to update
links.

Ambassadors are seen as a way to mitigate this, but as used in the example they
are only marginally useful in a multi-host setup and much less useful in a single
host scenario.

### Solution
The solution will very likey be added in Docker at some point, but until that
time, we need something a bit more dynamic.

Grand Ambassador reads all the exposed ports of the passed in container and
creates a proxy for each of those ports on all interfaces in the ambassador.  
Once the ambassador is started it will begin to monitor the Docker event stream
for potential changes to these settings and adjust the proxy settings
accordingly, without restarting the ambassador container.

### Usage
```bash
docker run -d -v /var/run/docker.sock:/docker.sock \
  cpuguy83/docker-grand-ambassador \
  -name container_name \
  -sock /docker.sock
```

```bash
Usage of /usr/bin/grand-ambassador:
  -log-level="info": Set debug logging
  -name=[]: Name/ID of container to ambassadorize
  -sock="/var/run/docker.sock": Path to docker socket
  -tls=false: Enable TLS for connecting to Docker socket
  -tlscacert="/root/.docker/ca.pem": Path to TLS ca cert
  -tlscert="/root/.docker/cert.pem": Path to TLS cert
  -tlskey="/root/.docker/key.pem": Path to TLS key
  -tlsverify=false: Enable TLS verification of the Docker host
  -wait=true: Wait for container to be created if it doesn't exist on start
```

### Example
```bash
docker run -d --expose 6379 --name redis redis
docker run -d -v /var/run/docker.sock:/var/run/docker.sock \
  --name redis_ambassador \
  cpuguy83/docker-grand-ambassador -name redis
docker run --rm --link redis_ambassador:db crosbymichael/redis-cli -h db ping
```

### Caveats

It's a proxy!
