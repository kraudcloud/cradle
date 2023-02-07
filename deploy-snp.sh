#!/bin/sh
ssh yca7h rm -rf /opt/kraud/cradle/pkg.old
ssh yca7h mv /opt/kraud/cradle/pkg /opt/kraud/cradle/pkg.old
rsync -avz ./ yca7h:/opt/kraud/cradle/ --exclude /build --exclude /test
