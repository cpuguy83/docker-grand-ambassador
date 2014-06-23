## Docker Grand Ambassador


This is a fully dynamic docker link ambassador.<br />
For information on link ambassadors see: http://docs.docker.com/articles/ambassador_pattern_linking/

This will proxy all requests to the requested container over all ports that are exposed.<br />
If the requested container is stopped, restarted, whatever, this will pick up that change and update the proxy to use the new IP address of the container.

### Usage

docker run -v /var/run/docker.sock:/docker.sock cpuguy83/docker-grand-ambassador -name container_name -sock /docker.sock

