[![Docker Build Status](http://72.14.176.28:49153/cpuguy83/docker-grand-ambassador)](https://registry.hub.docker.com/u/cpuguy83/docker-grand-ambassador)
## Docker Grand Ambassador

This is a fully dynamic docker link ambassador.<br />
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
creates a proxy for each of those ports on all interfaces in the ambassador.<br />
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

### Example
```bash
docker run -d --expose 6379 --name redis redis
docker run -d -v /var/run/docker.sock:/var/run/docker.sock \
  --name redis_ambassador \
  cpuguy83/docker-grand-ambassador -name redis
docker run --rm --link redis_ambassador:db crosbymichael/redis-cli -h db ping
```

### Caveats

Proxy is new and not heavily tested.

Currently UDP is not working properly in the proxy so this needs to be worked out.
