#!/bin/sh

THIS=`dirname $0`
cd $THIS/test


if [ ! -e cache.ext4.img ]; then
    dd if=/dev/zero of=cache.ext4.img bs=1M count=1000
fi

if [ ! -e swap.img ]; then
    dd if=/dev/zero of=swap.img bs=1M count=1000
fi

layer1=7b347a08-04e1-43a7-9c53-8ffb770a18fb

qemu-system-x86_64 \
    "-nographic" \
    "-no-acpi"  "-nodefaults" "-no-user-config"  "-nographic"  "-no-acpi"  "-enable-kvm"  "-no-reboot" \
    "-drive" 	"if=pflash,format=raw,unit=0,file=../pkg/pflash0" \
    "-cpu"      "host" \
    "-M"        "q35" \
    "-smp"      "2" \
    "-m"        "80M" \
    "-serial"   "stdio"\
    "-kernel"   "../pkg/kernel" \
    "-initrd"   "../pkg/initrd" \
    "-append"   "earlyprintk=ttyS0 console=ttyS0 reboot=t panic=-1 reboot=triple loglevel=4 ip=none" \
    "-device"   "virtio-net-pci,netdev=eth0" \
    "-netdev"   "user,id=eth0" \
    "-device"   "vhost-vsock-pci,id=vsock1,guest-cid=1123" \
    "-device"   "virtio-scsi,id=scsi0" \
    "-drive"    "format=raw,aio=threads,file=cache.ext4.img,readonly=off,if=none,id=drive-virtio-disk-cache" \
    "-device"   "scsi-hd,drive=drive-virtio-disk-cache,id=virtio-disk-cache,serial=cache" \
    "-drive"    "format=raw,aio=threads,file=swap.img,readonly=off,if=none,id=drive-virtio-disk-swap" \
    "-device"   "scsi-hd,drive=drive-virtio-disk-swap,id=virtio-disk-swap,serial=swap" \
    "-drive"    "format=raw,aio=threads,file=config.tar,readonly=off,if=none,id=drive-virtio-disk-config" \
    "-device"   "scsi-hd,drive=drive-virtio-disk-config,id=virtio-disk-config,serial=config" \
    "-watchdog" "i6300esb"  "-watchdog-action" "reset" \
    "-drive"    "format=raw,aio=threads,file=layer_$layer1,readonly=on,if=none,id=drive-virtio-layer1"  \
    "-device"   "scsi-hd,drive=drive-virtio-layer1,id=virtio-layer1,serial=layer.1,device_id=layer.$layer1" \
