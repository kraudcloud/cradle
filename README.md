cradle
======

the kraudcloud uvm.

./guest is the microvm userspace
./vmm is a vmm simulator that acts similar to a real vmm


## usage

    make
    docker save cradle:92d5f68  | docker --context kraud.aep load
    docker --context kraud.aep run -ti --label kr.cradle=cradle:92d5f68 busybox uname -a


## custom kernel

if you want to run a custom kernel config, drop it into kernel-config-x86_64 and type make.
note that the uvm depends on non standard kernel options, so ideally derive your config from the existing one

    make
    cd build/linux/
    make menuconfig
    cp .config ../../kernel-config-x86_64
    cd ../../
    make


there is no runtime loading, so all modules must be built in static.
if this is an issue, you could enable loading and bake modules into initrd yourself.


## secret encryption

customers who do not want to trust the HSM can encode their secrets with an extra key
before sending to k8s/docker apis and decode them inside guest/sec.go

secinit is post network, so you can contact an external server and do SEV-SNP remote attestation or other policy decisions.
If you contact any external service, you must make appopriate preparations to make sure it can scale with your workload.
There's a maximum boot time before a uvm is considered failed (about 1 second),
and if deployments do not settle within a redispatch window (about 15 minutes), they may be evicted.
