from alpine as build

run apk add go make gcc llvm clang

copy rebind46 /src/rebind46
run cd /src/rebind46 && make
run cp /src/rebind46/rebind46 /bin/rebind46

copy . /src

run cd /src/guest && go build -o /init


#----------------------------------------------------

from alpine as initrd
copy --from=build /init /init
copy --from=build /bin/rebind46 /bin/rebind46

run	ln -sf /init /sbin/init
run	ln -sf /init /bin/runc
run	ln -sf /init /bin/nsenter

run apk add --no-cache e2fsprogs xfsprogs cryptsetup nftables rsync
