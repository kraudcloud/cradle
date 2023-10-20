from alpine as build

run apk add go make gcc llvm clang

copy . /src

run cd /src/guest && go build -o /init


#----------------------------------------------------

from alpine as initrd
copy --from=build /init /init

run	ln -sf /init /sbin/init
run	ln -sf /init /bin/runc
run	ln -sf /init /bin/nsenter

run apk add --no-cache iproute2 e2fsprogs xfsprogs cryptsetup nftables rsync
