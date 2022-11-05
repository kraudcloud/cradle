#!/bin/sh

THIS=`dirname $0`
cd $THIS/test


layer1=layer.4451b8f2-1d33-48ba-8403-aba9559bb6af.tar.gz
volume1=volume.e4ee5e4a-ce31-47d6-a72e-f9e316439b5c.img

if [ ! -e cache.ext4.img ]; then
    dd if=/dev/zero of=cache.ext4.img bs=1M count=1000
fi

if [ ! -e swap.img ]; then
    dd if=/dev/zero of=swap.img bs=1M count=1000
fi

if [ ! -e $volume1 ]; then
    dd if=/dev/zero of=$volume1 bs=1M count=1000
fi


../vmm/vmm
