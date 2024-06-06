
VERSION := $(shell git describe --tags --always --dirty)


all:
	make ctr/cradle
	make ctr/cradle-snp

ctr/%:
	docker buildx build --progress=plain -t ctr.0x.pt/kraud/$(notdir $@):$(VERSION) -f Dockerfile --target $(notdir $@) . --push
