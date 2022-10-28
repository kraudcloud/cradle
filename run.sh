#!/bin/sh

THIS=`dirname $0`
cd $THIS/out


if [ ! -e cache.ext4.img ]; then
    dd if=/dev/zero of=cache.ext4.img bs=1M count=1000
fi

if [ ! -e swap.img ]; then
    dd if=/dev/zero of=swap.img bs=1M count=1000
fi

if [ ! -e initrd ]; then
( cd ../build/initrd && find . | cpio -o -H newc ) > initrd
fi


layer1=52dc2d59-a883-435a-8159-e3e407719f6c

qemu-system-x86_64 \
    "-nographic" \
    "-no-acpi"  "-nodefaults" "-no-user-config"  "-nographic"  "-no-acpi"  "-enable-kvm"  "-no-reboot" \
    "-bios"     "bios.bin" \
    "-cpu"      "host" \
    "-M"        "q35" \
    "-smp"      "2" \
    "-m"        "2G" \
    "-serial"   "stdio"\
    "-kernel"   "kernel" \
    "-initrd"   "initrd" \
    "-append"   "earlyprintk=ttyS0 console=ttyS0 reboot=t panic=-1 reboot=triple loglevel=6 ip=none" \
    "-device"   "virtio-serial" \
    "-chardev"  "socket,path=cradle,server=on,wait=off,id=cradle" \
    "-device"   "virtserialport,chardev=cradle,name=cradle" \
    "-device"   "virtio-net-pci,netdev=eth0" \
    "-netdev"   "user,id=eth0" \
    "-device"   "vhost-vsock-pci,id=vsock1,guest-cid=1123" \
    "-drive"    "format=raw,aio=threads,file=cache.ext4.img,readonly=off,if=none,id=drive-virtio-disk-cache" \
    "-device"   "virtio-blk-pci,scsi=off,drive=drive-virtio-disk-cache,id=virtio-disk-cache,serial=cache" \
    "-drive"    "format=raw,aio=threads,file=swap.img,readonly=off,if=none,id=drive-virtio-disk-swap" \
    "-device"   "virtio-blk-pci,scsi=off,drive=drive-virtio-disk-swap,id=virtio-disk-swap,serial=swap" \
    "-drive"    "format=raw,aio=threads,file=config.tar,readonly=off,if=none,id=drive-virtio-disk-config" \
    "-device"   "virtio-blk-pci,scsi=off,drive=drive-virtio-disk-config,id=virtio-disk-config,serial=config" \
    "-watchdog" "i6300esb"  "-watchdog-action" "reset" \
    "-device"   "virtio-scsi,id=scsi0" \
    "-drive"    "format=raw,aio=threads,file=layer_$layer1.tar,readonly=on,if=none,id=drive-virtio-layer1"  \
    "-device"   "scsi-hd,drive=drive-virtio-layer1,id=virtio-layer1,serial=$layer1,device_id=layer.$layer1" \
