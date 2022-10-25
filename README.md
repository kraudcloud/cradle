cradle
======

the kraudcloud microvm stack

runs any docker container inside qemu for complete isolation


 - guest     is the microvm userspace
 - vmm       is the host interface
 - host      contains a simple comandline for running a vmm host locally


## usage


    make

    docker pull ubuntu
    ./kcradle summon /tmp/testvm ubuntu
    ./kcradle run /tmp/testvm


now you can interact with vdocker from the host

	export DOCKER_HOST='tcp://[fddd::2]:1'
	docker ps
	docker exec -ti container.0 /bin/sh


