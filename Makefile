all: pkg.tar

pkg.tar: pkg/bios.bin pkg/kernel pkg/initrd test/config.tar
	cd pkg &&\
	docker build . -t cradle:$$(git describe --tags --always)

build/qboot:
	mkdir -p build
	cd build &&\
	git clone https://github.com/bonzini/qboot.git

pkg/bios.bin: build/qboot
	cd build/qboot &&\
	git checkout 8ca302e86d685fa05b16e2b208888243da319941 &&\
	meson build && ninja -C build
	cp build/qboot/build/bios.bin pkg/bios.bin

build/linux:
	mkdir -p build
	cd build &&\
	git clone https://github.com/torvalds/linux.git &&\

pkg/kernel: build/linux kernel-config-x86_64
	cd build/linux &&\
	git checkout v6.1-rc2 &&\
	cp ../../kernel-config-x86_64 .config &&\
	make oldconfig &&\
	make -j20
	cp build/linux/arch/x86_64/boot/bzImage pkg/kernel

pkg/initrd: build/initrd/init build/initrd/bin/busybox build/initrd/bin/mkfs.ext4
	( cd build/initrd && find . | cpio -o -H newc ) > pkg/initrd

test/config.tar: launch/cradle.json
	tar  cf test/config.tar -C launch .

build/busybox:
	mkdir -p build/
	cd build/&&\
	git clone git://git.busybox.net/busybox

build/initrd/bin/busybox: build/busybox
	cd build/busybox &&\
	make defconfig &&\
	sed -i .config -e 's/# CONFIG_STATIC is not set/CONFIG_STATIC=y/' &&\
	make -j &&\
	make CONFIG_PREFIX=../initrd install

build/initrd/init: .PHONY
	mkdir -p build/initrd
	cd guest && CGO_ENABLED=0 go build -tags nethttpomithttp2  -ldflags="-s -w" -o ../build/initrd/init -asmflags -trimpath
	mkdir -p build/initrd/bin
	ln -sf ../init build/initrd/bin/runc


build/e2fsprogs-1.46.5:
	mkdir -p build
	cd build &&\
	wget https://mirrors.edge.kernel.org/pub/linux/kernel/people/tytso/e2fsprogs/v1.46.5/e2fsprogs-1.46.5.tar.gz &&\
	tar -xzf e2fsprogs-1.46.5.tar.gz

build/initrd/bin/mkfs.ext4: build/e2fsprogs-1.46.5
	cd build/e2fsprogs-1.46.5 &&\
	LIBS=-static ./configure --enable-static --disable-shared &&\
	make -j &&\
	cp misc/mke2fs ../initrd/bin/mkfs.ext4


.PHONY:
