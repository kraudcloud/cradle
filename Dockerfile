from alpine as build

run apk add go make gcc llvm clang

run mkdir /src/
copy go.mod go.sum /src/
copy spec /src/spec
copy yeet /src/yeet
run cd /src/ &&  go mod download
copy guest /src/guest

run cd /src/guest && go build -o /init


#----------------------------------------------------

from alpine as initrd
copy --from=build /init /init

run	ln -sf /init /sbin/init

run apk add --no-cache iproute2 e2fsprogs xfsprogs cryptsetup nftables rsync

# TODO: these are enclaive specific. remove them once they havea  sidecar
run apk add --no-cache lvm2 cryptsetup sfdisk sgdisk e2fsprogs-extra
