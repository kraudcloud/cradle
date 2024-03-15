VARIANT=snp

all: kcradle pkg.tar


kcradle: .PHONY
	cd host &&\
	go build -race -o ../kcradle

pkg.tar: pkg-$(VARIANT)/pflash0 pkg-$(VARIANT)/kernel pkg-$(VARIANT)/initrd test/config.tar
	cd pkg-$(VARIANT) &&\
	docker build . -t cradle:$$(git describe --tags --always)-$(VARIANT)

build/qboot:
	mkdir -p build
	cd build &&\
	git clone https://github.com/bonzini/qboot.git

build/.firmware-default: build/qboot
	cd build/qboot &&\
	git checkout 8ca302e86d685fa05b16e2b208888243da319941 &&\
	meson build && ninja -C build
	cp build/qboot/build/bios.bin pkg-$(VARIANT)/pflash0
	touch $@

build/ovmf:
	mkdir -p build
	cd build &&\
	git clone https://github.com/AMDESE/ovmf.git &&\
	cd ovmf &&\
	git checkout snp-latest &&\
	git submodule update --init --recursive

build/.firmware-snp: build/ovmf
	cd build/ovmf &&\
	make -C BaseTools &&\
	bash -c '. ./edksetup.sh --reconfig &&  build --cmd-len=64436 -n20 -t GCC5 -a X64 -p OvmfPkg/OvmfPkgX64.dsc'
	cp -f build/ovmf/Build/OvmfX64/RELEASE_*/FV/OVMF_CODE.fd pkg-$(VARIANT)/pflash0
	cp -f build/ovmf/Build/OvmfX64/RELEASE_*/FV/OVMF_VARS.fd pkg-$(VARIANT)/pflash1
	touch $@

pkg-$(VARIANT)/pflash0: build/.firmware-$(VARIANT)

build/linux-snp:
	mkdir -p build
	cd build &&\
	git clone https://github.com/kraudcloud/amd-snp-kernel.git linux-snp --single-branch --branch snp-host-latest-tmp

build/linux-default:
	mkdir -p build
	cd build &&\
	git clone https://kernel.googlesource.com/pub/scm/linux/kernel/git/stable/linux linux-default --single-branch --branch linux-6.6.y &&\
	cd linux-default &&\
	git checkout v6.6.11

pkg-$(VARIANT)/kernel: build/linux-$(VARIANT) kernel-config-x86_64
	cd build/linux-$(VARIANT) &&\
	cp ../../kernel-config-x86_64 .config
	if [ "$(VARIANT)" = "snp" ]; \
	then \
	cd build/linux-$(VARIANT) &&\
	cp ../../kernel-config-x86_64-snp .config &&\
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
	./scripts/config --enable  CGROUP_MISC; \
	fi
	cd build/linux-$(VARIANT) &&\
	make olddefconfig &&\
	make -j8
	cp build/linux-$(VARIANT)/arch/x86_64/boot/bzImage pkg-$(VARIANT)/kernel

build/initrd/init: Dockerfile .PHONY
	rm -rf build/initrd
	mkdir -p build/initrd
	docker buildx build . -o - | tar -x -C build/initrd

build/initrd/lib/modules:  pkg-$(VARIANT)/kernel
	mkdir -p build/initrd
	cd build/linux-$(VARIANT) &&\
	make modules -j8 && make INSTALL_MOD_STRIP=1 INSTALL_MOD_PATH=../initrd modules_install

pkg-$(VARIANT)/initrd: build/initrd/init build/initrd/lib/modules
	( cd build/initrd && find . | cpio -o -H newc ) > pkg-$(VARIANT)/initrd

test/config.tar: launch/launch.json
	mkdir -p test
	tar  cf test/config.tar -C launch .




.PHONY:


upload: all
	cd pkg-$(VARIANT) &&\
	docker build . -t cradle:$$(git describe --tags --always)-$(VARIANT) &&\
	docker save cradle:$$(git describe --tags --always)-$(VARIANT) | docker --context=kraud.aep load


