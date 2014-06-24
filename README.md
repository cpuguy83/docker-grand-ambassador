## Docker Grand Ambassador


This is a fully dynamic docker link ambassador.<br />
For information on link ambassadors see: http://docs.docker.com/articles/ambassador_pattern_linking/

This will proxy all requests to the requested container over all ports that are exposed.<br />
If the requested container is stopped, restarted, whatever, this will pick up that change and update the proxy to use the new IP address of the container.

### Usage
```bash
docker run -d -v /var/run/docker.sock:/docker.sock \
  cpuguy83/docker-grand-ambassador \
  -name container_name \
  -sock /docker.sock
```


Grand Ambassador reads all the exposed ports of the passed in container and
creates a proxy for each of those ports on all interfaces in the ambassador.<br />
Once the ambassador is started it will the begin to monitor the Docker event
stream for potential changes to these settings.

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
