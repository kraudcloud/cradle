VARIANT=snp

all: vmm/vmm pkg.tar


vmm/vmm: .PHONY
	cd vmm/simulator &&\
	go build -race

pkg.tar: pkg/pflash0 pkg/kernel pkg/initrd test/config.tar
	cd pkg &&\
	docker build . -t cradle:$$(git describe --tags --always)-$(VARIANT)

build/qboot:
	mkdir -p build
	cd build &&\
	git clone https://github.com/bonzini/qboot.git

build/.firmware-default: build/qboot
	cd build/qboot &&\
	git checkout 8ca302e86d685fa05b16e2b208888243da319941 &&\
	meson build && ninja -C build
	cp build/qboot/build/bios.bin pkg/pflash0
	touch $@

build/ovmf:
	mkdir -p build
	cd build &&\
	git clone https://github.com/tianocore/edk2.git ovmf &&\
	cd ovmf &&\
	git checkout edk2-stable202208 &&\
	git submodule update --init --recursive

build/.firmware-snp: build/ovmf
	cd build/ovmf &&\
	make -C BaseTools &&\
	bash -c '. ./edksetup.sh --reconfig &&  build -b RELEASE -q --cmd-len=64436 -n20 -t GCC5 -a X64 -p OvmfPkg/OvmfPkgX64.dsc'
	cp -f build/ovmf/Build/OvmfX64/RELEASE_*/FV/OVMF_CODE.fd pkg/pflash0
	cp -f build/ovmf/Build/OvmfX64/RELEASE_*/FV/OVMF_VARS.fd pkg/pflash1
	touch $@

pkg/pflash0: build/.firmware-$(VARIANT)

build/linux:
	mkdir -p build
	cd build &&\
	git clone https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git --single-branch --branch linux-6.0.y &&\
	cd linux &&\
	git checkout v6.0.7

pkg/kernel: build/linux kernel-config-x86_64
	cd build/linux &&\
	cp ../../kernel-config-x86_64 .config &&\
	make oldconfig &&\
	make -j8
	cp build/linux/arch/x86_64/boot/bzImage pkg/kernel

pkg/initrd: build/initrd/init build/initrd/usr/sbin/cryptsetup
	( cd build/initrd && find . | cpio -o -H newc ) > pkg/initrd

test/config.tar: launch/launch.json
	mkdir -p test
	tar  cf test/config.tar -C launch .

#build/busybox:
#	mkdir -p build/
#	cd build/&&\
#	git clone git://git.busybox.net/busybox
#
#build/initrd/bin/busybox: build/busybox busybox-config-x86_64
#	cd build/busybox &&\
#	cp ../../busybox-config-x86_64 .config &&\
#	make oldconfig &&\
#	sed -i .config -e 's/# CONFIG_STATIC is not set/CONFIG_STATIC=y/' &&\
#	make -j &&\
#	make CONFIG_PREFIX=../initrd install

build/initrd/init: .PHONY build/initrd/usr/sbin/cryptsetup
	mkdir -p build/initrd
	cd guest &&\
	export BUILDROOT=$(PWD)/build/buildroot-2022.08.1/ &&\
	export CGO_CFLAGS="-Os -I$${BUILDROOT}/output/staging/include" &&\
	export CGO_LDFLAGS="-Os -L$${BUILDROOT}/output/staging/lib -L -L$${SYSROOT}/output/staging/usr/lib" &&\
	export CGO_ENABLED=1 &&\
	export CC=$${BUILDROOT}/output/host/bin/x86_64-buildroot-linux-musl-gcc &&\
	go build -tags nethttpomithttp2 -tags $(VARIANT) -ldflags="-s -w -linkmode external" -o ../build/initrd/init -asmflags -trimpath
	mkdir -p build/initrd/bin
	ln -sf ../init build/initrd/bin/runc
	ln -sf ../init build/initrd/bin/nsenter


build/e2fsprogs-1.46.5:
	mkdir -p build
	cd build &&\
	wget https://mirrors.edge.kernel.org/pub/linux/kernel/people/tytso/e2fsprogs/v1.46.5/e2fsprogs-1.46.5.tar.gz &&\
	tar -xzf e2fsprogs-1.46.5.tar.gz

#build/initrd/bin/mkfs.ext4: build/e2fsprogs-1.46.5
#	cd build/e2fsprogs-1.46.5 &&\
#	LIBS=-static ./configure --enable-static --disable-shared &&\
#	make -j &&\
#	cp misc/mke2fs ../initrd/bin/mkfs.ext4

build/buildroot-2022.08.1:
	mkdir -p build
	cd build &&\
	wget https://buildroot.org/downloads/buildroot-2022.08.1.tar.gz &&\
	tar -xzf buildroot-2022.08.1.tar.gz


build/initrd/usr/sbin/cryptsetup: build/buildroot-2022.08.1 buildroot-config-x86_64
	unset PERL_MM_OPT &&\
	cd build/buildroot-2022.08.1 &&\
	cp ../../buildroot-config-x86_64 .config &&\
	make oldconfig &&\
	make -j8 &&\
	mkdir -p ../initrd/usr/sbin/ &&\
	mkdir -p ../initrd/usr/lib/ &&\
	rsync -a output/target/lib/ ../initrd/lib/ &&\
	rsync -a output/target/usr/lib/ ../initrd/usr/lib/ &&\
	rsync -a output/target/bin/ ../initrd/bin/ &&\
	rsync -a output/target/sbin/ ../initrd/sbin/ &&\
	rsync -a output/target/usr/sbin/ ../initrd/usr/sbin/ &&\
	rsync -a output/target/usr/bin/ ../initrd/usr/bin/ &&\
	ln -sf /bin/ash ../initrd/bin/sh




.PHONY:

