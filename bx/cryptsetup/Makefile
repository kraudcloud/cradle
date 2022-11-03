all: main


# download

SRC_MUSL=build/musl-1.2.3/Makefile
SRC_KERNELHEADERS=build/linux-headers-4.19.88-1/README.md
SRC_CRYPTSETUP=build/cryptsetup-2.5.0/configure
SRC_UTILLINUX=build/util-linux-2.38.1/configure
SRC_LVM2=build/lvm2-2.02.188/configure
SRC_LIBAIO=build/libaio-libaio-0.3.11/Makefile
SRC_POPT=build/popt-1.19/configure
SRC_JSONC=build/json-c-0.15/CMakeLists.txt


$(SRC_MUSL):
	mkdir -p build
	cd build && wget -q https://www.musl-libc.org/releases/musl-1.2.3.tar.gz
	cd build && tar -xf musl-1.2.3.tar.gz

$(SRC_KERNELHEADERS):
	mkdir -p build
	cd build &&\
	wget https://github.com/sabotage-linux/kernel-headers/releases/download/v4.19.88-1/linux-headers-4.19.88-1.tar.xz &&\
	tar xf linux-headers-4.19.88-1.tar.xz

$(SRC_CRYPTSETUP):
	mkdir -p build
	cd build &&\
	wget https://mirrors.edge.kernel.org/pub/linux/utils/cryptsetup/v2.5/cryptsetup-2.5.0.tar.xz &&\
	tar -xJf cryptsetup-2.5.0.tar.xz

$(SRC_UTILLINUX):
	mkdir -p build
	cd build &&\
	wget https://mirrors.edge.kernel.org/pub/linux/utils/util-linux/v2.38/util-linux-2.38.1.tar.gz &&\
	tar -xzf util-linux-2.38.1.tar.gz

$(SRC_LVM2):
	mkdir -p build
	cd build &&\
	git clone https://sourceware.org/git/lvm2.git --single-branch --branch v2_02_188 --depth 1 lvm2-2.02.188 &&\
	cd lvm2-2.02.188 &&\
	patch -p0 < ../../mallinfo.patch &&\
	patch -p0 < ../../fix-stdio-usage.patch &&\
	patch -p0 < ../..//mlockall-default-config.patch

$(SRC_LIBAIO):
	mkdir -p build
	cd build &&\
	wget https://pagure.io/libaio/archive/libaio-0.3.112/libaio-libaio-0.3.11.tar.gz &&\
	tar -xzf libaio-libaio-0.3.11.tar.gz

$(SRC_POPT):
	mkdir -p build
	cd build &&\
	wget "http://ftp.rpm.org/popt/releases/popt-1.x/popt-1.19.tar.gz" &&\
	tar -xzf popt-1.19.tar.gz

$(SRC_JSONC):
	mkdir -p build
	cd build &&\
	wget https://s3.amazonaws.com/json-c_releases/releases/json-c-0.15.tar.gz &&\
	tar -xzf json-c-0.15.tar.gz


# build



MUSL=staging/usr/lib/libc.a
KERNEL_HEADERS=staging/usr/include/linux
LIBUUID=staging/usr/lib/libuuid.a
LIBCRYPTSETUP=staging/usr/lib/libcryptsetup.a
LIBDEVICEMAPPER=staging/usr/lib/libdevmapper.a
LIBAIO=staging/usr/lib/libaio.a
POPT=staging/usr/lib/libpopt.a
JSONC=staging/usr/lib/libjson-c.a

main:  $(LIBCRYPTSETUP)

$(MUSL): $(SRC_MUSL)
	mkdir -p staging
	cd build/musl-1.2.3 &&\
	./configure --disable-shared --prefix=$(CURDIR)/staging/usr/ &&\
	make -j$(JOBS) &&\
	make install

$(KERNEL_HEADERS): $(MUSL) $(SRC_KERNELHEADERS)
	mkdir -p staging
	cd build/linux-headers-4.19.88-1 &&\
	make ARCH=x86_64 prefix=$(PWD)/staging/usr/ install

$(LIBUUID): $(SRC_CRYPTSETUP) $(KERNEL_HEADERS)
	mkdir -p staging
	export CC=$(PWD)/staging/usr/bin/musl-gcc &&\
	cd build/util-linux-2.38.1 &&\
	./configure  --prefix=$(PWD)/staging/usr/ --disable-shared --enable-static --disable-all-programs --enable-libuuid --enable-libblkid &&\
	make -j &&\
	make install

$(LIBAIO): $(SRC_LIBAIO)
	mkdir -p staging
	export CC=$(PWD)/staging/usr/bin/musl-gcc &&\
	cd build/libaio-libaio-0.3.11 &&\
	make -j &&\
	make DESTDIR=$(PWD)/staging install

$(LIBDEVICEMAPPER): $(SRC_LVM2) $(LIBUUID) $(LIBAIO)
	export CC=$(PWD)/staging/usr/bin/musl-gcc &&\
	cd build/lvm2-2.02.188  &&\
	./configure  --prefix=$(PWD)/staging/usr/ --sysconfdir=$(PWD)/staging/etc --disable-shared --enable-static --enable-static_link --enable-pkgconfig \
		--disable-selinux --disable-readline --disable-use-lvmlockd --disable-use-lvmpolld \
		--disable-udev-systemd-background-jobs --disable-fsadm --disable-blkdeactivate \
		-disable-nls --with-thin=internal &&\
	make -j &&\
	make confdir=$(PWD)/staging/etc install

$(POPT): $(SRC_POPT)
	mkdir -p staging
	export PKG_CONFIG_LIBDIR=$(PWD)/staging/usr/lib/pkgconfig &&\
	export PKG_CONFIG_SYSROOT_DIR=$(PWD)/staging &&\
	export CC=$(PWD)/staging/usr/bin/musl-gcc &&\
	cd build/popt-1.19 &&\
	./configure  --prefix=$(PWD)/staging/usr/ --disable-shared --enable-static &&\
	make -j &&\
	make confdir=$(PWD)/staging/etc install

$(JSONC): $(SRC_JSONC)
	mkdir -p staging
	export CC=$(PWD)/staging/usr/bin/musl-gcc &&\
	cd build/json-c-0.15 &&\
	cmake -B build -G Ninja \
		-DCMAKE_INSTALL_PREFIX=$(PWD)/staging/usr/ \
		-DCMAKE_INSTALL_LIBDIR=lib \
		-DBUILD_SHARED_LIBS=False \
		-DBUILD_STATIC_LIBS=True \
		-DCMAKE_BUILD_TYPE=MinSizeRel &&\
	cmake --build build &&\
	cmake --build build --target doc &&\
	cmake --build build --target install

$(LIBCRYPTSETUP): $(SRC_UTILLINUX) $(LIBUUID) $(LIBDEVICEMAPPER) $(POPT) $(JSONC)
	export PKG_CONFIG_LIBDIR=$(PWD)/staging/usr/lib/pkgconfig &&\
	export PKG_CONFIG_SYSROOT_DIR=$(PWD)/staging &&\
	export CC=$(PWD)/staging/usr/bin/musl-gcc &&\
	export LD=$(PWD)/staging/usr/bin/musl-gcc &&\
	cd build/cryptsetup-2.5.0 &&\
	./configure  --prefix=$(PWD)/staging/usr/ --disable-shared --enable-static --disable-asciidoc --enable-static-cryptsetup --disable-udev \
	--disable-selinux --disable-udev --disable-ssh-token  --disable-nls --disable-veritysetup --disable-integritysetup \
	--with-crypto_backend=kernel &&\
	make -j &&\
	make install


