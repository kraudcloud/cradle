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



qemu-system-x86_64 \
    "-nographic" "-nodefaults" "-no-user-config"  "-nographic"  "-enable-kvm"  "-no-reboot" "-no-acpi" \
    "-cpu"      "host" \
    "-M"        "microvm,x-option-roms=off,pit=off,pic=off,isa-serial=off,rtc=off" \
    "-smp"      "2" \
    "-m"        "128M" \
    "-chardev"  "stdio,id=virtiocon0" \
    "-device"   "virtio-serial-device" \
    "-device"   "virtconsole,chardev=virtiocon0" \
    "-bios"     "../pkg/pflash0" \
    "-kernel"   "../pkg/kernel" \
    "-initrd"   "../pkg/initrd" \
    "-append"   "earlyprintk=hvc0 console=hvc0 loglevel=5" \
    "-device"   "virtio-net-device,netdev=eth0" \
    "-netdev"   "user,id=eth0" \
    "-device"   "vhost-vsock-device,id=vsock1,guest-cid=1123" \
    "-device"   "virtio-scsi-device,id=scsi0" \
    "-drive"    "format=raw,aio=threads,file=cache.ext4.img,readonly=off,if=none,id=drive-virtio-disk-cache" \
    "-device"   "virtio-blk-device,drive=drive-virtio-disk-cache,id=virtio-disk-cache,serial=cache" \
    "-drive"    "format=raw,aio=threads,file=swap.img,readonly=off,if=none,id=drive-virtio-disk-swap" \
    "-device"   "virtio-blk-device,drive=drive-virtio-disk-swap,id=virtio-disk-swap,serial=swap" \
    "-drive"    "format=raw,aio=threads,file=config.tar,readonly=off,if=none,id=drive-virtio-disk-config" \
    "-device"   "virtio-blk-device,drive=drive-virtio-disk-config,id=virtio-disk-config,serial=config" \
    "-drive"    "format=raw,aio=threads,file=$layer1,readonly=on,if=none,id=drive-virtio-layer1"  \
    "-device"   "scsi-hd,drive=drive-virtio-layer1,id=virtio-layer1,serial=layer.1,device_id=$layer1" \
    "-drive"    "format=raw,aio=threads,file=$volume1,readonly=off,if=none,id=drive-virtio-volume1"  \
    "-device"   "scsi-hd,drive=drive-virtio-volume1,id=virtio-volume1,serial=volume.1,device_id=$volume1" \
