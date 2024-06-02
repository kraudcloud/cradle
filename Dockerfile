#----------------------------------------------------
# build the kernel and modules
#----------------------------------------------------

from alpine as kernelbuilder

run apk --no-cache add git

workdir /src
run git clone git://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git --single-branch --branch linux-6.9.y

workdir /src/linux
run git checkout v6.9.3

copy kernel-config-x86_64 /src/linux/.config

run apk --no-cache add gcc make bison flex elfutils-dev libelf musl-dev libgcc diffutils linux-headers python3 findutils perl tar pahole

run make olddefconfig
run make -j8
run mkdir -p /pkg/default/
run cp arch/x86_64/boot/bzImage /pkg/default/kernel
run make modules -j8 && make INSTALL_MOD_STRIP=1 INSTALL_MOD_PATH=/pkg/default/root modules_install

run apk --no-cache add bash

run \
	./scripts/config --enable  EXPERT &&\
	./scripts/config --enable  DEBUG_INFO &&\
	./scripts/config --enable  AMD_MEM_ENCRYPT &&\
	./scripts/config --disable AMD_MEM_ENCRYPT_ACTIVE_BY_DEFAULT &&\
	./scripts/config --enable  KVM &&\
	./scripts/config --enable  KVM_AMD &&\
	./scripts/config --enable  PCI &&\
	./scripts/config --enable  CRYPTO_DEV_CCP_DD &&\
	./scripts/config --enable  KVM_AMD_SEV &&\
	./scripts/config --disable SYSTEM_TRUSTED_KEYS &&\
	./scripts/config --disable SYSTEM_REVOCATION_KEYS &&\
	./scripts/config --enable  SEV_GUEST &&\
	./scripts/config --disable IOMMU_DEFAULT_PASSTHROUGH &&\
	./scripts/config --disable PREEMPT_COUNT &&\
	./scripts/config --disable PREEMPTION &&\
	./scripts/config --disable PREEMPT_DYNAMIC &&\
	./scripts/config --disable DEBUG_PREEMPT &&\
	./scripts/config --enable  CGROUP_MISC


run make olddefconfig
run make -j8
run mkdir -p /pkg/snp/
run cp arch/x86_64/boot/bzImage /pkg/snp/kernel
run make modules -j8 && make INSTALL_MOD_STRIP=1 INSTALL_MOD_PATH=/pkg/snp/root modules_install

#----------------------------------------------------
# build the firmware, qboot for default, and OVMF for snp
#----------------------------------------------------

from alpine as firmwarebuilder
workdir /src

run apk --no-cache add git gcc make bison flex elfutils-dev libelf ninja meson libgcc  musl-dev

run \
  git clone https://github.com/bonzini/qboot.git &&\
  cd qboot &&\
  git checkout 8ca302e86d685fa05b16e2b208888243da319941 &&\
  meson build && ninja -C build &&\
  mkdir -p /pkg/default/ &&\
  cp build/bios.bin /pkg/default/pflash0

run \
  mkdir -p /src &&\
  git clone https://github.com/AMDESE/ovmf.git &&\
  cd ovmf &&\
  git checkout snp-latest &&\
  git checkout 4b6ee06a090d956f80b4a92fb9bf03098a372f39 &&\
  git submodule update --init --recursive

run apk add --no-cache util-linux-dev bash g++ nasm iasl

workdir /src/ovmf
run make -C BaseTools
run bash -c '. ./edksetup.sh --reconfig &&  build --cmd-len=64436 -n20 -t GCC5 -a X64 -p OvmfPkg/OvmfPkgX64.dsc -b RELEASE'

run mkdir -p /pkg/snp
run ls -lisah Build/OvmfX64/
run cp -f Build/OvmfX64/RELEASE_*/FV/OVMF_CODE.fd /pkg/snp/pflash0
run cp -f Build/OvmfX64/RELEASE_*/FV/OVMF_VARS.fd /pkg/snp/pflash1

#----------------------------------------------------
# build cradle host and guest
#----------------------------------------------------

from alpine as gobuild

run apk add go make gcc llvm clang

workdir /src/
copy . /src/

run cd /src/guest && go build -o init
run cd /src && go build -o cradle


#----------------------------------------------------
# build initrd, which contains the cradle guest, and modules
#----------------------------------------------------

from alpine as initrd-default

run apk add --no-cache iproute2 e2fsprogs xfsprogs cryptsetup nftables rsync

copy --from=gobuild /src/guest/init /init
run ln -sf /init /sbin/init

copy --from=kernelbuilder /pkg/default/root/lib/modules /lib/modules

#----------------------------------------------------

from alpine as initrd-snp

run apk add --no-cache iproute2 e2fsprogs xfsprogs cryptsetup nftables rsync
# TODO: these are enclaive specific. remove them once they have a sidecar
run apk add --no-cache lvm2 cryptsetup sfdisk sgdisk e2fsprogs-extra

copy --from=gobuild /src/guest/init /init
run ln -sf /init /sbin/init
run ls -lisah /init

copy --from=kernelbuilder /pkg/snp/root/lib/modules /lib/modules

#----------------------------------------------------
# build the final runner image
#----------------------------------------------------

from alpine as ctr-build-default

run apk --no-cache add iproute2 zfs qemu-system-x86_64 virtiofsd nftables tcpdump docker-cli findutils
copy --from=gobuild /src/cradle /bin/cradle
entrypoint ["/bin/cradle"]

run mkdir -p /cradle/
copy --from=kernelbuilder 	/pkg/default/kernel 		/cradle/kernel
copy --from=firmwarebuilder /pkg/default/pflash0 		/cradle/pflash0
run cat - > /cradle/cradle.json <<EOF
{
    "version": 2,
    "firmware": {
        "pflash0": "pflash0"
    },
    "kernel": {
        "kernel":   "kernel",
        "initrd":   "initrd"
    },
    "machine": {
        "type": "microvm"
    }
}
EOF


copy --from=initrd-default / /build/initrd
run mknod -m 666 /build/initrd/dev/console c 5 1
run mknod -m 666 /build/initrd/dev/null c 1 3
run ( cd /build/initrd && find . -mount  | cpio -o -H newc ) > /cradle/initrd
run rm -rf /build/

from scratch as cradle
copy --from=ctr-build-default / /

#----------------------------------------------------

from alpine as ctr-build-snp

run apk --no-cache add iproute2 zfs nftables tcpdump docker-cli findutils
copy --from=gobuild /src/cradle /bin/cradle
entrypoint ["/bin/cradle"]

run mkdir -p /cradle/
copy --from=kernelbuilder 	/pkg/snp/kernel 		/cradle/kernel
copy --from=firmwarebuilder /pkg/snp/pflash0 		/cradle/pflash0
copy --from=firmwarebuilder /pkg/snp/pflash1 		/cradle/pflash1
run cat - > /cradle/cradle.json <<EOF
{
    "version": 2,
    "firmware": {
        "pflash0": "pflash0",
        "pflash1": "pflash1"
    },
    "kernel": {
        "kernel":   "kernel",
        "initrd":   "initrd"
    },
    "machine": {
        "type": "snp"
    }
}
EOF

copy --from=initrd-snp / /build/initrd
run mknod -m 666 /build/initrd/dev/console c 5 1
run mknod -m 666 /build/initrd/dev/null c 1 3
run ( cd /build/initrd && find . -mount | cpio -o -H newc ) > /cradle/initrd
run rm -rf /build/

run apk --no-cache add go zfs-dev make git ninja pkgconfig pixman-dev glib-dev wget liburing-dev libcap-ng-dev attr-dev bash perl python3 gcc g++

run git clone https://github.com/AMDESE/qemu.git
run cd qemu &&\
    git checkout snp-latest &&\
    git checkout fb924a5139bff1d31520e007ef97b616af1e22a1 &&\
    ./configure \
    --target-list=x86_64-softmmu  \
    --enable-debug-info \
    --enable-tpm \
    --enable-virtfs \
    --enable-linux-io-uring \
    --extra-cflags=-Wno-error \
    --prefix=/ &&\
    make -j &&\
    make install

from scratch as cradle-snp
copy --from=ctr-build-snp / /
